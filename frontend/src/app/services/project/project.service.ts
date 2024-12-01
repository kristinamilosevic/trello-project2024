import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Project } from '../../models/project/project';

@Injectable({
  providedIn: 'root'
})
export class ProjectService {

  updateTaskStatus(id: any, status: any) {
    throw new Error('Method not implemented.');
  }

  private apiUrl = 'http://localhost:8003/api/projects';


  private mainUrl = 'http://localhost:8003/api';

  private addUrl = 'http://localhost:8003/api/projects/add';


  constructor(private http: HttpClient) {}

  createProject(projectData: { name: string; expectedEndDate: string; minMembers: number; maxMembers: number }): Observable<any> {
    const token = localStorage.getItem('token'); 
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`, 
    });
    return this.http.post(this.addUrl, projectData, { headers });
  }
  

  getProjects(): Observable<Project[]> {
    return this.http.get<Project[]>(`${this.apiUrl}/all`);
  }

  getProjectById(id: string): Observable<Project> {
    return this.http.get<Project>(`${this.apiUrl}/${id}`);
  }
  
  getTasksForProject(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/${projectId}/tasks`);
  }

  getProjectsByUsername(username: string): Observable<Project[]> {
    return this.http.get<Project[]>(`${this.apiUrl}?username=${username}`);
  }

  
  
}  