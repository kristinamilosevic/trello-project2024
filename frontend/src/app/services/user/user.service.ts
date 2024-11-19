import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class UserService {
  private apiUrl = 'http://localhost:8001/api/users/change-password'; // Endpoint za promenu lozinke

  constructor(private http: HttpClient) { }

  changePassword(oldPassword: string, newPassword: string, confirmPassword: string): Observable<any> {
    const token = localStorage.getItem('token'); // Pretpostavljamo da je JWT token sačuvan u localStorage

    if (!token) {
      throw new Error('Token is missing!');
    }

    const headers = new HttpHeaders().set('Authorization', `Bearer ${token}`);

    const body = {
      oldPassword,
      newPassword,
      confirmPassword
    };

    return this.http.post(this.apiUrl, body, { headers });
  }
}
