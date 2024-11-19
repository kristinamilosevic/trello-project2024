import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})

export class TaskService {
  private apiUrl = 'http://localhost:8000/api/tasks';

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
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/project/${projectId}/available-members`;
    return this.http.get<any[]>(apiUrl);
  }
  
  // Dodaj članove na zadatak
  addMembersToTask(taskId: string, members: any[]): Observable<any> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/add-members`;
    return this.http.post(apiUrl, members);
  }
  
  getTaskMembers(taskId: string): Observable<any> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/members`;
    return this.http.get(apiUrl);
  }
  
}
