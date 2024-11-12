import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Project } from '../../models/project/project';

@Injectable({
  providedIn: 'root'
})
export class ProjectService {
  private apiUrl = 'http://localhost:8003/projects';
  private mainUrl = 'http://localhost:8003';

  constructor(private http: HttpClient) {}

  createProject(projectData: { name: string; expectedEndDate: string; minMembers: number; maxMembers: number }): Observable<any> {
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Manager-ID': '507f191e810c19729de860ea'
    });
    return this.http.post(this.apiUrl, projectData, { headers });
  }

  getProjects(): Observable<Project[]> {
    return this.http.get<Project[]>(this.apiUrl);
  }

  getProjectById(id: string): Observable<Project> {
    return this.http.get<Project>(`${this.apiUrl}/${id}`);
  }
  
  getTasksForProject(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/${projectId}/tasks`);
  }

  getProjectsByUsername(username: string): Observable<Project[]> {
    return this.http.get<Project[]>(`${this.mainUrl}/projects?username=${username}`);
  }
  
}  
