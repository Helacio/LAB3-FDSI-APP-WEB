import { Component, input } from '@angular/core';

@Component({
  selector: 'app-command-catalog',
  templateUrl: './command-catalog.html',
})
export class CommandCatalog {
  readonly commands = input<string[]>([]);
}
