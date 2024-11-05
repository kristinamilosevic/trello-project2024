import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { Member } from '../../models/member/member.model';

@Injectable({
  providedIn: 'root'
})
export class ProjectMembersService {
  private apiUrl = 'http://localhost:8080';

  constructor(private http: HttpClient) {}

  getProjectMembers(projectId: string): Observable<Member[]> {
    return this.http.get<Member[]>(`${this.apiUrl}/projects/${projectId}/members`).pipe(
      catchError((error) => {
        console.error('Error in getProjectMembers:', error);
        return throwError(error);
      })
    );
  }

  getAllUsers(): Observable<Member[]> {
    return this.http.get<Member[]>(`${this.apiUrl}/users`).pipe(
      catchError((error) => {
        console.error('Error in getAllUsers:', error);
        return throwError(error);
      })
    );
  }

  addMembers(projectId: string, memberIds: string[]): Observable<any> {
    return this.http.post(`${this.apiUrl}/projects/${projectId}/members`, memberIds).pipe(
      catchError((error) => {
        console.error('Error in addMembers:', error);
        return throwError(error);
      })
    );
  }

  removeMember(projectId: string, memberId: string): Observable<any> {
    return this.http.delete(`${this.apiUrl}/projects/${projectId}/members/${memberId}/remove`).pipe(
      catchError((error) => {
        console.error('Error in removeMember:', error);
        return throwError(error);
      })
    );
  }
  

}
