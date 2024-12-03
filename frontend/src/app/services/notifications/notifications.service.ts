import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class NotificationService {
  private apiUrl = 'http://localhost:8000/api/notifications';
  private getUrl = 'http://localhost:8000/api/notifications/all';

  constructor(private http: HttpClient) {}

  // Dohvati notifikacije korisnika
  getNotifications(username: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.getUrl}?username=${username}`);
  }

  // Označi notifikaciju kao pročitanu
  markAsRead(notificationId: string, username: string): Observable<any> {
    return this.http.put(`${this.apiUrl}/read`, { notificationId, username });
  }
}
