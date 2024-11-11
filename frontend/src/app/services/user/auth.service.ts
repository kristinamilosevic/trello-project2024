import { Injectable } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { Observable, BehaviorSubject } from 'rxjs';
import { tap } from 'rxjs/operators';
import { LoginRequest, LoginResponse } from '../../models/user/user';

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private apiUrl = 'http://localhost:8080';
  private loggedIn = new BehaviorSubject<boolean>(this.hasToken());

  constructor(private http: HttpClient) {}

  // Funkcija za prijavljivanje korisnika
  login(credentials: LoginRequest): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${this.apiUrl}/login`, credentials).pipe(
      tap((response: LoginResponse) => {
        // Čuvanje tokena u localStorage
        localStorage.setItem('token', response.token);
        localStorage.setItem('email', response.email);
        localStorage.setItem('role', response.role);
        this.loggedIn.next(true);
      })
    );
  }

  // Nova funkcija za potvrdu e-maila
  confirmEmail(token: string): Observable<any> {
    return this.http.post(`${this.apiUrl}/confirm`, { token });
  }
  

  // Provera da li je korisnik prijavljen
  isLoggedIn(): Observable<boolean> {
    return this.loggedIn.asObservable();
  }

  // Funkcija za odjavljivanje korisnika
  logout(): void {
    localStorage.removeItem('token');
    localStorage.removeItem('email');
    localStorage.removeItem('role');
    this.loggedIn.next(false);
  }

  // Dobijanje tokena iz localStorage
  getToken(): string | null {
    return localStorage.getItem('token');
  }

  // Dobijanje uloge korisnika
  getUserRole(): string | null {
    return localStorage.getItem('role');
  }

  // Funkcija za proveru da li postoji token u localStorage
  private hasToken(): boolean {
    return !!localStorage.getItem('token');
  }
}
