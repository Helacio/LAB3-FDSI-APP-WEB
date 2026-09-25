import { HttpClient } from '@angular/common/http';
import { computed, inject, Injectable, signal } from '@angular/core';
import { Observable, tap } from 'rxjs';
import { AuthTokenResponse, LoginResponse, Me } from './models';

const TOKEN_KEY = 'muv_token';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly http = inject(HttpClient);

  readonly autenticado = signal(false);
  readonly usuario = signal('');
  readonly rol = signal('');
  readonly esCambiador = computed(() => this.rol() === 'cambiador');

  token(): string | null {
    return localStorage.getItem(TOKEN_KEY);
  }

  init(): void {
    const token = this.token();
    if (!token) {
      return;
    }
    this.http.get<Me>('/api/auth/me').subscribe({
      next: (me) => {
        this.usuario.set(me.usuario);
        this.rol.set(me.rol);
        this.autenticado.set(true);
      },
      error: () => this.logout(),
    });
  }

  login(usuario: string, password: string): Observable<LoginResponse> {
    return this.http.post<LoginResponse>('/api/auth/login', { usuario, password });
  }

  verify(ticket: string, codigo: string): Observable<AuthTokenResponse> {
    return this.http
      .post<AuthTokenResponse>('/api/auth/verify', { ticket, codigo })
      .pipe(
        tap((resp) => {
          localStorage.setItem(TOKEN_KEY, resp.token);
          this.usuario.set(resp.usuario);
          this.rol.set(resp.rol);
          this.autenticado.set(true);
        }),
      );
  }

  logout(): void {
    localStorage.removeItem(TOKEN_KEY);
    this.autenticado.set(false);
    this.usuario.set('');
    this.rol.set('');
  }
}
