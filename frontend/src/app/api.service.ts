import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { AuthService } from './auth.service';
import {
  Device,
  Kpis,
  NewSession,
  Session,
  StatusCheck,
  StatusInfo,
  StatusRun,
  VerifyResult,
} from './models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
  private readonly auth = inject(AuthService);
  private readonly base = '/api';

  getStatus(): Observable<StatusInfo> {
    return this.http.get<StatusInfo>(`${this.base}/status`);
  }

  getKpis(): Observable<Kpis> {
    return this.http.get<Kpis>(`${this.base}/kpis`);
  }

  getInventory(): Observable<Device[]> {
    return this.http.get<Device[]>(`${this.base}/inventory`);
  }

  getCommands(): Observable<string[]> {
    return this.http.get<string[]>(`${this.base}/commands`);
  }

  getSessions(limit = 20): Observable<Session[]> {
    const params = new HttpParams().set('limit', limit);
    return this.http.get<Session[]>(`${this.base}/sessions`, { params });
  }

  verifySessions(): Observable<VerifyResult> {
    return this.http.get<VerifyResult>(`${this.base}/sessions/verify`);
  }

  createSession(payload: NewSession): Observable<Session> {
    return this.http.post<Session>(`${this.base}/sessions`, payload);
  }

  startStatusRun(): Observable<{ runId: string; total: number }> {
    return this.http.post<{ runId: string; total: number }>(`${this.base}/status/checks`, {});
  }

  getStatusRun(runId: string): Observable<{ run: StatusRun; checks: StatusCheck[] }> {
    return this.http.get<{ run: StatusRun; checks: StatusCheck[] }>(
      `${this.base}/status/runs/${runId}`,
    );
  }

  getRecentRuns(limit = 10): Observable<StatusRun[]> {
    const params = new HttpParams().set('limit', limit);
    return this.http.get<StatusRun[]>(`${this.base}/status/runs`, { params });
  }

  getLatestStatus(): Observable<StatusCheck[]> {
    return this.http.get<StatusCheck[]>(`${this.base}/status/latest`);
  }

  statusStreamUrl(runId: string): string {
    const token = this.auth.token() ?? '';
    return `${this.base}/status/stream?runId=${encodeURIComponent(runId)}&token=${encodeURIComponent(token)}`;
  }
}
