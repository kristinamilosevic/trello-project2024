import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})

export class TaskService {
  private apiUrl = 'http://localhost:8002/tasks';

  constructor(private http: HttpClient) {}

  createTask(taskData: { projectId: string; title: string; description: string }): Observable<any> {
    const headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    return this.http.post(this.apiUrl, taskData, { headers });
  }

  getAllTasks(): Observable<any[]> {
    return this.http.get<any[]>(this.apiUrl);
  }
  // Dohvati dostupne članove za dodavanje na task
  getAvailableMembers(projectId: string, taskId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/tasks/${taskId}/project/${projectId}/available-members`);
  }

  // Dodaj članove na zadatak
  addMembersToTask(taskId: string, members: any[]): Observable<any> {
    const headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    return this.http.post(`${this.apiUrl}/tasks/${taskId}/add-members`, members, { headers });
  }
  getTaskMembers(taskId: string): Observable<any[]> {
    return this.http.get<any[]>(`http://localhost:8002/tasks/${taskId}/members`);
  }
  
}
