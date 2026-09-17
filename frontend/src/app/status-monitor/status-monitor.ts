import { Component, inject, input, OnDestroy, OnInit, signal } from '@angular/core';
import { ApiService } from '../api.service';
import { Device, StatusCheck, StreamPayload } from '../models';

type Phase = 'idle' | 'running' | 'done' | 'error';

@Component({
  selector: 'app-status-monitor',
  templateUrl: './status-monitor.html',
})
export class StatusMonitor implements OnInit, OnDestroy {
  private readonly api = inject(ApiService);

  readonly devices = input<Device[]>([]);

  protected readonly phase = signal<Phase>('idle');
  protected readonly live = signal(false);
  protected readonly runId = signal('');
  protected readonly total = signal(0);
  protected readonly completados = signal(0);
  protected readonly results = signal<Record<string, StatusCheck>>({});
  protected readonly pending = signal<ReadonlySet<string>>(new Set());
  protected readonly summary = signal<Record<string, number> | null>(null);
  protected readonly error = signal('');

  private source?: EventSource;

  ngOnInit(): void {
    this.loadLatest();
  }

  ngOnDestroy(): void {
    this.closeStream();
  }

  protected start(): void {
    this.error.set('');
    this.summary.set(null);
    this.completados.set(0);
    this.pending.set(new Set(this.devices().map((d) => d.hostname)));
    this.phase.set('running');

    this.api.startStatusRun('portal').subscribe({
      next: ({ runId, total }) => {
        this.runId.set(runId);
        this.total.set(total);
        this.openStream(runId);
      },
      error: () => {
        this.phase.set('error');
        this.error.set('No se pudo iniciar la revisión (verifica backend y RabbitMQ).');
      },
    });
  }

  protected percent(): number {
    const total = this.total();
    return total > 0 ? Math.round((this.completados() / total) * 100) : 0;
  }

  protected isPending(hostname: string): boolean {
    return this.pending().has(hostname);
  }

  protected checkFor(hostname: string): StatusCheck | undefined {
    return this.results()[hostname];
  }

  protected estadoClass(estado: string): string {
    const value = (estado ?? '').toLowerCase();
    if (value.includes('fuera')) {
      return 'off';
    }
    if (value.includes('mantenimiento')) {
      return 'warn';
    }
    return 'ok';
  }

  protected formatLatency(check?: StatusCheck): string {
    return check && check.latenciaMs > 0 ? `${check.latenciaMs} ms` : '—';
  }

  protected formatTime(timestamp?: string): string {
    if (!timestamp) {
      return '—';
    }
    const date = new Date(timestamp);
    return isNaN(date.getTime()) ? timestamp : date.toLocaleTimeString();
  }

  protected summaryEntries(): [string, number][] {
    const summary = this.summary();
    return summary ? Object.entries(summary) : [];
  }

  private loadLatest(): void {
    this.api.getLatestStatus().subscribe({
      next: (checks) => {
        const map: Record<string, StatusCheck> = {};
        for (const check of checks) {
          map[check.hostname] = check;
        }
        this.results.set(map);
      },
    });
  }

  private openStream(runId: string): void {
    this.closeStream();
    const source = new EventSource(this.api.statusStreamUrl(runId));
    this.source = source;

    source.onopen = () => this.live.set(true);

    source.addEventListener('run.started', (event) => {
      const payload = this.parse(event);
      if (payload?.total) {
        this.total.set(payload.total);
      }
    });

    source.addEventListener('check.result', (event) => {
      const payload = this.parse(event);
      if (!payload?.hostname) {
        return;
      }
      const check: StatusCheck = {
        runId: payload.runId ?? runId,
        hostname: payload.hostname,
        gestion: '',
        estado: payload.estado ?? '',
        latenciaMs: payload.latenciaMs ?? 0,
        detalle: payload.detalle ?? '',
        timestamp: payload.timestamp ?? new Date().toISOString(),
      };
      this.results.update((map) => ({ ...map, [check.hostname]: check }));
      this.pending.update((set) => {
        const next = new Set(set);
        next.delete(check.hostname);
        return next;
      });
      if (typeof payload.completados === 'number') {
        this.completados.set(payload.completados);
      }
      if (typeof payload.total === 'number') {
        this.total.set(payload.total);
      }
    });

    source.addEventListener('run.finished', (event) => {
      const payload = this.parse(event);
      if (payload?.resumen) {
        this.summary.set(payload.resumen);
      }
      if (typeof payload?.completados === 'number') {
        this.completados.set(payload.completados);
      }
      this.phase.set('done');
      this.closeStream();
    });

    source.addEventListener('stream.error', (event) => {
      const payload = this.parse(event);
      this.error.set(payload?.error ?? 'El broker no está disponible.');
      this.phase.set('error');
      this.closeStream();
    });

    source.onerror = () => {
      // EventSource reintenta solo; solo reflejamos que se perdió el vivo.
      if (this.phase() === 'running') {
        this.live.set(false);
      }
    };
  }

  private parse(event: Event): StreamPayload | null {
    try {
      return JSON.parse((event as MessageEvent).data) as StreamPayload;
    } catch {
      return null;
    }
  }

  private closeStream(): void {
    if (this.source) {
      this.source.close();
      this.source = undefined;
    }
    this.live.set(false);
  }
}
