import { Injectable } from '@angular/core';
import { CanActivate, Router } from '@angular/router';
import { AuthService } from '../services/user/auth.service';

@Injectable({
  providedIn: 'root',
})
export class AuthGuard implements CanActivate {
  constructor(private authService: AuthService, private router: Router) {}

  canActivate(): boolean {
    if (this.authService.isLoggedIn()) { // Proverite da li korisnik ima validnu prijavu
      return true;
    } else {
      this.router.navigate(['/login']); // Ako nije prijavljen, preusmerite na login stranicu
      return false;
    }
  }
}
