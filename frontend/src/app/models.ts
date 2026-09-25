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
  operador?: string;
}

export interface LoginResponse {
  estado: 'enroll' | 'mfa';
  usuario?: string;
  secreto?: string;
  url?: string;
  ticket: string;
}

export interface AuthTokenResponse {
  token: string;
  usuario: string;
  rol: string;
  expiraEn: string;
}

export interface Me {
  usuario: string;
  rol: string;
}

export interface VerifyResult {
  ok: boolean;
  total: number;
  legadas: number;
  primerRompimiento: string;
}

export interface StatusRun {
  runId: string;
  operador: string;
  total: number;
  completados: number;
  estado: string;
  iniciadoEn: string;
  finalizadoEn?: string;
}

export interface StatusCheck {
  runId: string;
  hostname: string;
  gestion: string;
  estado: string;
  latenciaMs: number;
  detalle: string;
  timestamp: string;
}

export interface StatusEvent {
  type: string;
  runId: string;
  operador?: string;
  hostname?: string;
  estado?: string;
  latenciaMs?: number;
  detalle?: string;
  completados?: number;
  total?: number;
  resumen?: Record<string, number>;
  timestamp: string;
}

export type StreamPayload = Partial<StatusEvent> & { error?: string };
