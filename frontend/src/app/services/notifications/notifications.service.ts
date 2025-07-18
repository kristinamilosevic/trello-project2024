import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class NotificationService {
  private apiUrl = 'http://localhost:8000/api/notifications';
  private getUrl = 'http://localhost:8000/api/notifications/all';

  constructor(private http: HttpClient) {}
  private getAuthHeaders(): HttpHeaders {
    const token = localStorage.getItem('token'); 
    const role = localStorage.getItem('role'); 
    if (!token || !role) {
      throw new Error('Token or Role is missing!');
    }

    return new HttpHeaders()
      .set('Authorization', `Bearer ${token}`)
      .set('Role', role); 
  }

  getNotifications(username: string): Observable<any[]> {
    const headers = this.getAuthHeaders();
    return this.http.get<any[]>(`${this.getUrl}?username=${username}`, { headers });
  }


  markAsRead(notificationId: string, username: string): Observable<any> {
    const headers = this.getAuthHeaders();
    const body = { notificationId, username };
    return this.http.put(`${this.apiUrl}/read`, body, { headers });
  }
}
