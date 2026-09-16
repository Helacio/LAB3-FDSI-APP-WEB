export interface Device {
  hostname: string;
  tipo: string;
  sitio: string;
  rol: string;
  gestion: string;
  estado: string;
}

export interface Kpis {
  dispositivosGestionados: number;
  comandosPermitidos: number;
  sesionesHoy: number;
  propietario: string;
}

export interface StatusInfo {
  entorno: string;
  propietario: string;
  aviso: string;
}

export interface Session {
  dispositivo: string;
  comando: string;
  operador: string;
  timestamp: string;
  resultado: string;
}

export interface NewSession {
  dispositivo: string;
  comando: string;
  operador: string;
}
