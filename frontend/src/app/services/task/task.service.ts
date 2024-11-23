import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})

export class TaskService {
  private apiUrl = 'http://localhost:8000/api/tasks';
  private projectUrl = 'http://localhost:8000/api/projects';


  constructor(private http: HttpClient) {}

  createTask(taskData: { projectId: string; title: string; description: string; }): Observable<any> {
    const headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    return this.http.post((`${this.apiUrl}/create`), taskData, { headers });
  }

  getAllTasks(): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/all`);
    
  }
  
  getTasksByProject(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/project/${projectId}`);
  }

  getTasksForProject(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.projectUrl}/${projectId}/tasks`);
  }
  
  
  updateTaskStatus(taskId: string, status: string): Observable<any> {
    const url = `${this.apiUrl}/status`; 
    const username = localStorage.getItem('username'); 
    const body = { taskId, status, username }; 
  
    console.log('Sending request to update status:', body);
  
    return this.http.post(url, body, {
      headers: new HttpHeaders({ 'Content-Type': 'application/json' }) 
    });
  }


  getAvailableMembers(projectId: string, taskId: string): Observable<any[]> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/project/${projectId}/available-members`;
    return this.http.get<any[]>(apiUrl);
  }
  

  addMembersToTask(taskId: string, members: any[]): Observable<any> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/add-members`;
    return this.http.post(apiUrl, members);
  }
  
  getTaskMembers(taskId: string): Observable<any> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/members`;
    return this.http.get(apiUrl);
  }

  removeMemberFromTask(taskId: string, memberId: string): Observable<any> {
    const apiUrl = `http://localhost:8002/api/tasks/${taskId}/members/${memberId}`;  
    const token = localStorage.getItem('token');  

    const headers = new HttpHeaders().set('Authorization', `Bearer ${token}`).set('Content-Type', 'application/json');  

    return this.http.delete(apiUrl, { headers });
}

  
}
