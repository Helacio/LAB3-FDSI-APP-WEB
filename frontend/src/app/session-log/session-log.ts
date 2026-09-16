import { Component, inject, input, output, signal } from '@angular/core';
import { ApiService } from '../api.service';
import { Device, Session } from '../models';

@Component({
  selector: 'app-session-log',
  templateUrl: './session-log.html',
})
export class SessionLog {
  private readonly api = inject(ApiService);

  readonly sessions = input<Session[]>([]);
  readonly devices = input<Device[]>([]);
  readonly commands = input<string[]>([]);
  readonly created = output<void>();

  protected readonly dispositivo = signal('');
  protected readonly comando = signal('');
  protected readonly operador = signal('');
  protected readonly sending = signal(false);
  protected readonly message = signal('');
  protected readonly error = signal('');

  protected submit(event: Event): void {
    event.preventDefault();
    this.message.set('');
    this.error.set('');

    const dispositivo = this.dispositivo().trim();
    const comando = this.comando().trim();
    if (!dispositivo || !comando) {
      this.error.set('Selecciona un dispositivo y un comando.');
      return;
    }

    this.sending.set(true);
    this.api
      .createSession({ dispositivo, comando, operador: this.operador().trim() })
      .subscribe({
        next: () => {
          this.sending.set(false);
          this.message.set('Orden registrada en la bitácora.');
          this.created.emit();
        },
        error: () => {
          this.sending.set(false);
          this.error.set('No se pudo registrar la orden en el backend.');
        },
      });
  }

  protected formatTime(timestamp: string): string {
    const date = new Date(timestamp);
    return isNaN(date.getTime()) ? timestamp : date.toLocaleString();
  }
}
