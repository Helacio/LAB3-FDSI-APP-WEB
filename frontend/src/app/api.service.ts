import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { Device, Kpis, NewSession, Session, StatusInfo } from './models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  private readonly http = inject(HttpClient);
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

  createSession(payload: NewSession): Observable<Session> {
    return this.http.post<Session>(`${this.base}/sessions`, payload);
  }
}
