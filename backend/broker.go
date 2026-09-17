package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	jobsQueueName  = "router.status.jobs"
	eventsExchange = "router.status.events"
)

// statusJob es una unidad de trabajo: revisar el estado de un dispositivo.
type statusJob struct {
	RunID       string `json:"runId"`
	Hostname    string `json:"hostname"`
	Gestion     string `json:"gestion"`
	Operador    string `json:"operador"`
	RequestedAt string `json:"requestedAt"`
}

// statusEvent es el progreso que se publica por el exchange fanout y llega al SSE.
type statusEvent struct {
	Type        string         `json:"type"` // run.started | check.result | run.finished
	RunID       string         `json:"runId"`
	Operador    string         `json:"operador,omitempty"`
	Hostname    string         `json:"hostname,omitempty"`
	Estado      string         `json:"estado,omitempty"`
	LatenciaMs  int            `json:"latenciaMs,omitempty"`
	Detalle     string         `json:"detalle,omitempty"`
	Completados int            `json:"completados,omitempty"`
	Total       int            `json:"total,omitempty"`
	Resumen     map[string]int `json:"resumen,omitempty"`
	Timestamp   string         `json:"timestamp"`
}

// Broker abstrae el message broker para no acoplar el resto del código a AMQP.
type Broker interface {
	PublishJob(ctx context.Context, job statusJob) error
	ConsumeJobs(ctx context.Context, workers int, handler func(statusJob)) error
	PublishEvent(ctx context.Context, event statusEvent) error
	SubscribeEvents(ctx context.Context) (<-chan statusEvent, func(), error)
	Close() error
}

// rabbitBroker implementa Broker sobre RabbitMQ (AMQP 0-9-1).
//
// Topología:
//   - Cola durable "router.status.jobs": los workers compiten por los jobs.
//   - Exchange fanout "router.status.events": cada suscriptor SSE crea su propia
//     cola exclusiva/auto-delete, de modo que varios navegadores ven el mismo run.
type rabbitBroker struct {
	conn *amqp.Connection

	mu        sync.Mutex
	pubCh     *amqp.Channel
	pubClosed bool
}

func newRabbitBroker(uri string) (*rabbitBroker, error) {
	conn, err := amqp.Dial(uri)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := ch.QueueDeclare(jobsQueueName, true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := ch.ExchangeDeclare(eventsExchange, "fanout", true, false, false, false, nil); err != nil {
		conn.Close()
		return nil, err
	}
	return &rabbitBroker{conn: conn, pubCh: ch}, nil
}

func (b *rabbitBroker) PublishJob(ctx context.Context, job statusJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return b.publish(ctx, "", jobsQueueName, body, amqp.Persistent)
}

func (b *rabbitBroker) PublishEvent(ctx context.Context, event statusEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return b.publish(ctx, eventsExchange, "", body, amqp.Transient)
}

func (b *rabbitBroker) publish(ctx context.Context, exchange, key string, body []byte, mode uint8) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.pubClosed {
		return errors.New("canal de publicacion cerrado")
	}
	return b.pubCh.PublishWithContext(ctx, exchange, key, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: mode,
		Timestamp:    time.Now().UTC(),
		Body:         body,
	})
}

// ConsumeJobs reparte los jobs entre "workers" goroutines sobre la cola de trabajos.
// Cada job se confirma (ack) solo despues de que el handler termina.
func (b *rabbitBroker) ConsumeJobs(ctx context.Context, workers int, handler func(statusJob)) error {
	if workers < 1 {
		workers = 1
	}
	ch, err := b.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.Qos(workers, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(jobsQueueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case d, ok := <-deliveries:
					if !ok {
						return
					}
					var job statusJob
					if err := json.Unmarshal(d.Body, &job); err != nil {
						log.Printf("job invalido, descartado: %v", err)
						_ = d.Nack(false, false)
						continue
					}
					handler(job)
					_ = d.Ack(false)
				}
			}
		}()
	}
	wg.Wait()
	return nil
}

// SubscribeEvents crea una cola exclusiva ligada al exchange fanout. Cada llamada
// recibe TODOS los eventos (fan-out), lo que permite multiples clientes SSE.
func (b *rabbitBroker) SubscribeEvents(ctx context.Context) (<-chan statusEvent, func(), error) {
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, nil, err
	}
	q, err := ch.QueueDeclare("", false, true, true, false, nil)
	if err != nil {
		ch.Close()
		return nil, nil, err
	}
	if err := ch.QueueBind(q.Name, "", eventsExchange, false, nil); err != nil {
		ch.Close()
		return nil, nil, err
	}
	deliveries, err := ch.Consume(q.Name, "", true, true, false, false, nil)
	if err != nil {
		ch.Close()
		return nil, nil, err
	}

	out := make(chan statusEvent, 64)
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			ch.Close()
		})
	}

	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				cancel()
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				var ev statusEvent
				if err := json.Unmarshal(d.Body, &ev); err != nil {
					continue
				}
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				default:
					// Cliente lento: se descarta para no bloquear el fan-out.
				}
			}
		}
	}()

	return out, cancel, nil
}

func (b *rabbitBroker) Close() error {
	b.mu.Lock()
	b.pubClosed = true
	b.mu.Unlock()
	if b.pubCh != nil {
		_ = b.pubCh.Close()
	}
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}
