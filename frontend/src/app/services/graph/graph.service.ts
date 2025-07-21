import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class GraphService {
  private apiUrl = 'http://localhost:8000/api/graph';


  constructor(private http: HttpClient) {}

  getGraph(projectId: string): Observable<any> {
    const token = localStorage.getItem('token');
    const role = localStorage.getItem('role');

    const headers = new HttpHeaders({
      'Authorization': `Bearer ${token}`,
      'role': role || ''
    });

    return this.http.get<any>(`${this.apiUrl}/${projectId}`, { headers });
  }
}
