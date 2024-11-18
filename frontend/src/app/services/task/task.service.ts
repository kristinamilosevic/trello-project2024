import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})

export class TaskService {
  private apiUrl = 'http://localhost:8002/tasks';


  constructor(private http: HttpClient) {}

  createTask(taskData: { projectId: string; title: string; description: string; }): Observable<any> {
    const headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    return this.http.post(this.apiUrl, taskData, { headers });
  }

  getAllTasks(): Observable<any[]> {
    return this.http.get<any[]>(this.apiUrl);
    
  }
  
  getTasksByProject(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/project/${projectId}`);
  }
  
  
  updateTaskStatus(taskId: string, status: string): Observable<any> {
    const url = `${this.apiUrl}/status`;
    const body = { taskId, status };
    console.log('Sending request to update status:', body);
    return this.http.post(url, body, {
      headers: new HttpHeaders({ 'Content-Type': 'application/json' })
    });
  }
  
}
