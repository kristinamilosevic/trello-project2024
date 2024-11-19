import { Component } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { UserService } from '../../services/user/user.service';
import { Router } from '@angular/router';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-change-password',
  standalone: true, 
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './change-password.component.html',
  styleUrls: ['./change-password.component.css']
})
export class ChangePasswordComponent {
  changePasswordForm: FormGroup;

  constructor(private fb: FormBuilder, private userService: UserService, private router: Router) {
    this.changePasswordForm = this.fb.group({
      oldPassword: ['', [Validators.required]],
      newPassword: ['', [Validators.required, Validators.minLength(6)]],
      confirmPassword: ['', [Validators.required]]
    });
  }

  onSave() {
    if (this.changePasswordForm.valid) {
      const { oldPassword, newPassword, confirmPassword } = this.changePasswordForm.value;

      if (newPassword !== confirmPassword) {
        alert('Nova lozinka i potvrda lozinke se ne poklapaju!');
        return;
      }

      // Pozivanje UserService-a za promenu lozinke
      this.userService.changePassword(oldPassword, newPassword, confirmPassword).subscribe({
        next: (response) => {
          alert('Lozinka je uspešno promenjena!');
          // Nakon uspešne promene lozinke, preusmeravamo korisnika na njegov profil
          this.router.navigate(['/users-profile']);
        },
        error: (error) => {
          console.error('Error:', error);
          alert('Greška pri promeni lozinke: ' + (error.error?.message || error.message || 'Unknown error'));
        }
      });
    } else {
      alert('Popunite sva polja ispravno!');
    }
  }
}
