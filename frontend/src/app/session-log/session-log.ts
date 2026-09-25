import { Component, inject, input, output, signal } from '@angular/core';
import { ApiService } from '../api.service';
import { AuthService } from '../auth.service';
import { Device, Session } from '../models';

@Component({
  selector: 'app-session-log',
  templateUrl: './session-log.html',
})
export class SessionLog {
  private readonly api = inject(ApiService);
  protected readonly auth = inject(AuthService);

  readonly sessions = input<Session[]>([]);
  readonly devices = input<Device[]>([]);
  readonly commands = input<string[]>([]);
  readonly created = output<void>();

  protected readonly dispositivo = signal('');
  protected readonly comando = signal('');
  protected readonly sending = signal(false);
  protected readonly message = signal('');
  protected readonly error = signal('');
  protected readonly verifying = signal(false);
  protected readonly verResult = signal('');

  protected verificar(): void {
    this.verifying.set(true);
    this.verResult.set('');
    this.api.verifySessions().subscribe({
      next: (result) => {
        this.verifying.set(false);
        if (result.ok) {
          this.verResult.set(
            `Integridad OK: ${result.total} entradas verificadas` +
              (result.legadas > 0 ? ` (${result.legadas} legadas sin sello)` : '') +
              '.',
          );
        } else {
          this.verResult.set(
            `ALERTA: cadena rota. Primer rompimiento en ${result.primerRompimiento || 'entrada sin timestamp'}.`,
          );
        }
      },
      error: () => {
        this.verifying.set(false);
        this.verResult.set('No se pudo verificar la bitácora.');
      },
    });
  }

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
      .createSession({ dispositivo, comando })
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
