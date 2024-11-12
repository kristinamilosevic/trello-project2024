import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class AccountService {
  private apiUrl = 'http://localhost:8080/api/auth/delete-account';

  constructor(private http: HttpClient) {}

  deleteAccount(username: string, role: string): Observable<any> {
    const token = localStorage.getItem('token'); // Uzimamo token iz localStorage
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`
    });

    const url = `${this.apiUrl}/${username}/${role}`;
    return this.http.delete(url, { headers });
  }
}
