import { Component, inject, signal } from '@angular/core';
import { forkJoin } from 'rxjs';
import { ApiService } from './api.service';
import { AuthService } from './auth.service';
import { CommandCatalog } from './command-catalog/command-catalog';
import { InventoryTable } from './inventory-table/inventory-table';
import { Login } from './login/login';
import { Device, Kpis, Session, StatusInfo } from './models';
import { SessionLog } from './session-log/session-log';
import { StatusMonitor } from './status-monitor/status-monitor';
import { ThemeToggle } from './theme-toggle/theme-toggle';

@Component({
  selector: 'app-root',
  imports: [ThemeToggle, InventoryTable, CommandCatalog, SessionLog, StatusMonitor, Login],
  styleUrl: './app.css',
  templateUrl: './app.html',
})
export class App {
  private readonly api = inject(ApiService);
  protected readonly auth = inject(AuthService);

  protected readonly status = signal<StatusInfo | null>(null);
  protected readonly kpis = signal<Kpis | null>(null);
  protected readonly inventory = signal<Device[]>([]);
  protected readonly commands = signal<string[]>([]);
  protected readonly sessions = signal<Session[]>([]);
  protected readonly state = signal<'loading' | 'ready' | 'error'>('loading');
  protected readonly error = signal('');

  constructor() {
    this.auth.init();
    if (this.auth.token()) {
      this.load();
    }
  }

  protected onLogged(): void {
    this.load();
  }

  protected logout(): void {
    this.auth.logout();
  }

  protected load(): void {
    this.state.set('loading');
    this.error.set('');
    forkJoin({
      status: this.api.getStatus(),
      kpis: this.api.getKpis(),
      inventory: this.api.getInventory(),
      commands: this.api.getCommands(),
      sessions: this.api.getSessions(),
    }).subscribe({
      next: ({ status, kpis, inventory, commands, sessions }) => {
        this.status.set(status);
        this.kpis.set(kpis);
        this.inventory.set(inventory);
        this.commands.set(commands);
        this.sessions.set(sessions);
        this.state.set('ready');
      },
      error: () => {
        this.error.set(
          'No se pudo contactar el backend. Verifica que el servicio Go y MongoDB estén en ejecución.',
        );
        this.state.set('error');
      },
    });
  }

  protected onSessionCreated(): void {
    forkJoin({
      kpis: this.api.getKpis(),
      sessions: this.api.getSessions(),
    }).subscribe({
      next: ({ kpis, sessions }) => {
        this.kpis.set(kpis);
        this.sessions.set(sessions);
      },
    });
  }
}
