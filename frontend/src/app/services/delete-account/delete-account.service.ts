import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class AccountService {
  private apiUrl = 'http://localhost:8000/api/users/auth/delete-account';

  constructor(private http: HttpClient) {}

  // Funkcija za kreiranje zaglavlja sa tokenom i rodom
  private getAuthHeaders(): HttpHeaders {
    const token = localStorage.getItem('token'); // Uzimanje tokena iz localStorage
    const role = localStorage.getItem('role'); // Uzimanje uloge iz localStorage
    if (!token || !role) {
      throw new Error('Token or Role is missing'); // Bacanje greške ako token ili role ne postoji
    }

    return new HttpHeaders({
      Authorization: `Bearer ${token}`, // Postavljanje Authorization header-a
      Role: role, // Dodavanje Role header-a
    });
  }

  // Metoda za brisanje korisničkog naloga
  deleteAccount(): Observable<any> {
    const headers = this.getAuthHeaders(); // Kreiranje zaglavlja
    return this.http.delete(this.apiUrl, { headers }); // Slanje DELETE zahteva sa zaglavljem
  }
}
