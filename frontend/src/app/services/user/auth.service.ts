import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable, BehaviorSubject } from 'rxjs';
import { tap } from 'rxjs/operators';
import { LoginRequest, LoginResponse } from '../../models/user/user';

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  private apiUrl = 'http://localhost:8001/api/users';
  private loggedIn = new BehaviorSubject<boolean>(this.hasToken());

  constructor(private http: HttpClient) {}

  // Funkcija za generisanje zaglavlja sa tokenom i rodom
  private getAuthHeaders(): HttpHeaders {
    const token = localStorage.getItem('token');
    const role = localStorage.getItem('role');
    
    if (!token || !role) {
      throw new Error('Token or Role is missing!');
    }

    // Vraća zaglavlje sa tokenom i rodom
    return new HttpHeaders().set('Authorization', `Bearer ${token}`).set('Role', role);
  }

  // Funkcija za prijavljivanje korisnika
  login(credentials: LoginRequest): Observable<LoginResponse> {
    return this.http.post<LoginResponse>(`${this.apiUrl}/login`, credentials).pipe(
      tap((response: LoginResponse) => {
        // Čuvanje tokena i role u localStorage
        localStorage.setItem('token', response.token);
        localStorage.setItem('username', response.username);
        localStorage.setItem('role', response.role);
        this.loggedIn.next(true);
      })
    );
  }

  // Funkcija za slanje linka za reset lozinke
  sendPasswordResetLink(username: string, email: string): Observable<any> {
    const headers = this.getAuthHeaders();
    return this.http.post(`${this.apiUrl}/forgot-password`, { username, email }, { headers });
  }

  // Funkcija za slanje magičnog linka
  sendMagicLink(username: string, email: string): Observable<any> {
    const headers = this.getAuthHeaders();
    return this.http.post(`${this.apiUrl}/magic-link`, { username, email }, { headers });
  }

  // Funkcija za verifikaciju magičnog linka
  verifyMagicLink(token: string): Observable<any> {
    return this.http.get<any>(`${this.apiUrl}/magic-login?token=${token}`);
  }

  // Funkcija za proveru da li korisnik postoji
  checkUsername(username: string): Observable<any> {
    const headers = this.getAuthHeaders();
    return this.http.get(`${this.apiUrl}/check-username/${username}`, { headers });
  }

  // Provera da li je korisnik prijavljen
  isLoggedIn(): Observable<boolean> {
    return this.loggedIn.asObservable();
  }

  // Funkcija za odjavljivanje korisnika
  logout(): void {
    localStorage.removeItem('token');
    localStorage.removeItem('username');
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

  isAuthorized(roles: string[]): boolean {
    const userRole = this.getUserRole();
    return userRole ? roles.includes(userRole) : false;
  }

  // Funkcija za proveru da li postoji token u localStorage
  private hasToken(): boolean {
    return !!localStorage.getItem('token');
  }

  // Provera da li korisnik ima specifičnu rolu
  hasRole(role: string): boolean {
    const userRole = this.getUserRole();
    return userRole === role;
  }

  // Funkcija za dobijanje korisničkog profila
  getUserProfile(): Observable<any> {
    const headers = this.getAuthHeaders();
    return this.http.get<any>(`${this.apiUrl}/users-profile`, { headers });
  }
}
