import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class AccountService {
  private apiUrl = 'http://localhost:8080/api/auth/delete-account';

  constructor(private http: HttpClient) {}

  deleteAccount(userId: string, role: string): Observable<any> {
    const url = `${this.apiUrl}/${userId}/${role}`;
    return this.http.delete(url);
  }
}

