import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { catchError, Observable, throwError } from 'rxjs';
import { Project } from '../../models/project/project';

@Injectable({
  providedIn: 'root'
})
export class ProjectService {

  private apiUrl = 'http://localhost:8003/api/projects';
  private addUrl = 'http://localhost:8003/api/projects/add';

  constructor(private http: HttpClient) {}

  // Helper function to get headers with Authorization and Role
  private getHeadersWithRole(): HttpHeaders {
    const token = localStorage.getItem('token');
    const role = localStorage.getItem('role');
    
    if (!token || !role) {
      console.error('Token or Role missing');
      return new HttpHeaders(); // Return empty headers if missing
    }

    return new HttpHeaders({
      'Authorization': `Bearer ${token}`,
      'Role': role, // Add only the role in the header
    });
  }

  // Function to create a project with full headers (including Manager-ID)
  createProject(projectData: { name: string; expectedEndDate: string; minMembers: number; maxMembers: number }): Observable<any> {
    const token = localStorage.getItem('token');
    const role = localStorage.getItem('role');
    
    // Check if token and role are available
    if (!token || !role) {
      console.error('Token or Role missing');
      return throwError('Token or Role missing'); // Return error Observable if missing
    }

    // Set headers including Authorization, Role, and Manager-ID
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`, // Add Authorization header
      'Role': role, // Send role in the header
      'Manager-ID': '507f191e810c19729de860ea', // This can be dynamic if needed
    });

    return this.http.post(this.addUrl, projectData, { headers }).pipe(
      catchError(error => {
        console.error('Error in creating project:', error); // Log the error
        return throwError('Failed to create project'); // Return error Observable
      })
    );
  }

  // Function to get all projects, only with role in the header
  getProjects(): Observable<Project[]> {
    const headers = this.getHeadersWithRole(); // Get headers with only role
    return this.http.get<Project[]>(`${this.apiUrl}/all`, { headers }).pipe(
      catchError(error => {
        console.error('Error fetching projects:', error); // Log the error
        return throwError('Failed to fetch projects'); // Return error Observable
      })
    );
  }

  // Function to get a project by ID, only with role in the header
  getProjectById(id: string): Observable<Project> {
    const headers = this.getHeadersWithRole(); // Get headers with only role
    return this.http.get<Project>(`${this.apiUrl}/${id}`, { headers }).pipe(
      catchError(error => {
        console.error('Error fetching project by ID:', error); // Log the error
        return throwError('Failed to fetch project by ID'); // Return error Observable
      })
    );
  }

  // Function to get tasks for a project, only with role in the header
  getTasksForProject(projectId: string): Observable<any[]> {
    const headers = this.getHeadersWithRole(); // Get headers with only role
    return this.http.get<any[]>(`${this.apiUrl}/${projectId}/tasks`, { headers }).pipe(
      catchError(error => {
        console.error('Error fetching tasks for project:', error); // Log the error
        return throwError('Failed to fetch tasks for project'); // Return error Observable
      })
    );
  }

  // Function to get projects by username, only with role in the header
  getProjectsByUsername(username: string): Observable<Project[]> {
    const headers = this.getHeadersWithRole(); // Get headers with only role
    return this.http.get<Project[]>(`${this.apiUrl}?username=${username}`, { headers }).pipe(
      catchError(error => {
        console.error('Error fetching projects by username:', error); // Log the error
        return throwError('Failed to fetch projects by username'); // Return error Observable
      })
    );
  }
}
