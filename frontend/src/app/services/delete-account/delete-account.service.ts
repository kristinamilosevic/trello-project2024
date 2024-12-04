import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class AccountService {
  private apiUrl = 'http://localhost:8000/api/users/auth/delete-account';

  constructor(private http: HttpClient) {}

  private getAuthHeaders(): HttpHeaders {
    const token = localStorage.getItem('token'); 
    const role = localStorage.getItem('role'); 
    if (!token || !role) {
      throw new Error('Token or Role is missing');
    }

    return new HttpHeaders({
      Authorization: `Bearer ${token}`, 
      Role: role, 
    });
  }
  deleteAccount(): Observable<any> {
    const headers = this.getAuthHeaders(); 
    return this.http.delete(this.apiUrl, { headers });
  }
}