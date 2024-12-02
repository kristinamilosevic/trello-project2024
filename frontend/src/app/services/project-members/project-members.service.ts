import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { Member } from '../../models/member/member.model';
import { Project } from '../../models/project/project';

@Injectable({
  providedIn: 'root'
})
export class ProjectMembersService {
  private apiUrl = 'http://localhost:8003/api';

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
      'Role': role // Add role in the header
    });
  }

  getProjectMembers(projectId: string): Observable<Member[]> {
    const headers = this.getHeadersWithRole(); // Get headers with Authorization and Role
    return this.http.get<Member[]>(`${this.apiUrl}/projects/${projectId}/members`, { headers }).pipe(
      catchError((error) => {
        console.error('Error in getProjectMembers:', error);
        return throwError(error);
      })
    );
  }

  getAllUsers(): Observable<Member[]> {
    const headers = this.getHeadersWithRole(); // Get headers with Authorization and Role
    return this.http.get<Member[]>(`${this.apiUrl}/projects/users`, { headers }).pipe(
      catchError((error) => {
        console.error('Error in getAllUsers:', error);
        return throwError(error);
      })
    );
  }

  addMembers(projectId: string, memberIds: string[]): Observable<any> {
    const headers = this.getHeadersWithRole(); // Get headers with Authorization and Role
    headers.append('Content-Type', 'application/json'); // Add content type
    return this.http.post(`${this.apiUrl}/projects/${projectId}/members`, memberIds, { headers }).pipe(
      catchError((error) => {
        console.error('Error in addMembers:', error);
        return throwError(error);
      })
    );
  }

  removeMember(projectId: string, memberId: string): Observable<any> {
    const headers = this.getHeadersWithRole(); // Get headers with Authorization and Role
    return this.http.delete(`${this.apiUrl}/projects/${projectId}/members/${memberId}/remove`, { headers }).pipe(
      catchError((error) => {
        console.error('Error in removeMember:', error);
        return throwError(error);
      })
    );
  }

  getProjectDetails(projectId: string): Observable<Project> {
    const headers = this.getHeadersWithRole(); // Get headers with Authorization and Role
    return this.http.get<Project>(`${this.apiUrl}/projects/${projectId}`, { headers }).pipe(
      catchError((error) => {
        console.error('Error in getProjectDetails:', error);
        return throwError(error);
      })
    );
  }
}
