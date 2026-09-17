import { Component, signal } from '@angular/core';

type Theme = 'dark' | 'light' | '';

@Component({
  selector: 'app-theme-toggle',
  templateUrl: './theme-toggle.html',
})
export class ThemeToggle {
  protected readonly mode = signal<Theme>(this.readStored());

  constructor() {
    this.apply(this.mode());
  }

  protected toggle(): void {
    const next: Theme = this.mode() === 'dark' ? 'light' : 'dark';
    this.mode.set(next);
    localStorage.setItem('muv-theme', next);
    this.apply(next);
  }

  protected label(): string {
    return this.mode() === 'dark' ? 'Cambiar a modo día' : 'Cambiar a modo noche';
  }

  private readStored(): Theme {
    const stored = localStorage.getItem('muv-theme');
    return stored === 'dark' || stored === 'light' ? stored : '';
  }

  private apply(mode: Theme): void {
    const root = document.documentElement;
    root.classList.toggle('dark', mode === 'dark');
    root.classList.toggle('light', mode === 'light');
  }
}
