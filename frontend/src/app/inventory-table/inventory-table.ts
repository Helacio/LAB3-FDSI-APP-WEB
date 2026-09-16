import { Component, input } from '@angular/core';
import { Device } from '../models';

@Component({
  selector: 'app-inventory-table',
  templateUrl: './inventory-table.html',
})
export class InventoryTable {
  readonly devices = input<Device[]>([]);

  estadoClass(estado: string): string {
    const value = estado.toLowerCase();
    if (value.includes('fuera')) {
      return 'off';
    }
    if (value.includes('mantenimiento')) {
      return 'warn';
    }
    return 'ok';
  }
}
