import { Component, inject, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { AuthService } from '../auth.service';

@Component({
  selector: 'app-login',
  imports: [FormsModule],
  templateUrl: './login.html',
})
export class Login {
  private readonly auth = inject(AuthService);

  readonly logged = output<void>();

  protected readonly paso = signal<'creds' | 'mfa' | 'enroll'>('creds');
  protected readonly usuario = signal('');
  protected readonly password = signal('');
  protected readonly codigo = signal('');
  protected readonly ticket = signal('');
  protected readonly secreto = signal('');
  protected readonly url = signal('');
  protected readonly busy = signal(false);
  protected readonly error = signal('');

  protected submitCreds(): void {
    this.busy.set(true);
    this.error.set('');
    this.auth.login(this.usuario().trim(), this.password()).subscribe({
      next: (resp) => {
        this.busy.set(false);
        this.ticket.set(resp.ticket);
        if (resp.estado === 'mfa') {
          this.paso.set('mfa');
        } else {
          this.secreto.set(resp.secreto ?? '');
          this.url.set(resp.url ?? '');
          this.paso.set('enroll');
        }
      },
      error: (err) => {
        this.busy.set(false);
        const detalle = err?.error?.error ?? '';
        this.error.set(
          detalle === 'demasiados intentos, espera 5 minutos'
            ? 'Demasiados intentos fallidos. Espera 5 minutos.'
            : 'Credenciales inválidas.',
        );
      },
    });
  }

  protected submitCodigo(): void {
    this.busy.set(true);
    this.error.set('');
    this.auth.verify(this.ticket(), this.codigo().trim()).subscribe({
      next: () => {
        this.busy.set(false);
        this.logged.emit();
      },
      error: () => {
        this.busy.set(false);
        this.error.set('Código inválido o vencido. Intenta de nuevo.');
      },
    });
  }

  protected volver(): void {
    this.paso.set('creds');
    this.codigo.set('');
    this.error.set('');
  }
}
