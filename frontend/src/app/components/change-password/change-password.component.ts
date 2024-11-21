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
  errorMessage: string = '';

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
        alert('The new password and the confirmation password do not match!');
        return;
      }
  
      // Pozivanje UserService-a za promenu lozinke
      this.userService.changePassword(oldPassword, newPassword, confirmPassword).subscribe({
        next: () => {
          alert('Password changed successfully!');
          // Nakon uspešne promene lozinke, preusmeravamo korisnika na njegov profil
          this.router.navigate(['/users-profile']);
        },
        error: (error) => {
          console.error('Error:', error);
  
          // Hendlovanje specifičnih poruka grešaka
          if (error.status === 400 && error.error) {
            // Proveri da li poruka sadrži specifične ključne reči
            if (error.error.includes('old password is incorrect')) {
              alert('The old password is incorrect. Please try again.');
            } else if (error.error.includes('new password and confirmation do not match')) {
              alert('The new password and confirmation password do not match!');
            } else {
              alert('Error changing password: ' + error.error);
            }
          } else {
            alert('An unexpected error occurred. Please try again later.');
          }
        },
      });
    } else {
      alert('Please fill out all fields correctly!');
    }
  }
  
  
}
