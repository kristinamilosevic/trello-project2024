import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';


@Injectable({
  providedIn: 'root'
})
export class ProjectMembersService {
  private apiUrl = 'http://localhost:8080/projects';


  constructor(private http: HttpClient) {}

  getProjectMembers(projectId: string): Observable<any[]> {
    return this.http.get<any[]>(`${this.apiUrl}/${projectId}/members`).pipe(
      catchError((error) => {
        console.error('Error in getProjectMembers:', error);
        return throwError(error);
      })
    );
  }
  

  removeMember(projectId: string, memberId: string): Observable<any> {
    return this.http.delete(`${this.apiUrl}/617f1f77bcf86cd799439011/members/${memberId}/remove`);
  }
  
}

