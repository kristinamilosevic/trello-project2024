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
      newPassword: ['', [
          Validators.required,
          Validators.minLength(8),
          Validators.pattern('^(?=.*[A-Z])(?=.*\\d)(?=.*[!@#$%^&*.,])[A-Za-z\\d!@#$%^&*.,]{8,}$')]],
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
          this.router.navigate(['/users-profile']);
        },
        error: (error) => {
          console.error('Error:', error);
  
          if (error.status === 400 && error.error) {
            // Proveri da li poruka sadrži specifične ključne reči
            const errorMessage = typeof error.error === 'string' ? error.error : error.error.message;

          if (errorMessage) {
            if (errorMessage.includes('old password is incorrect')) {
              alert('The old password is incorrect. Please try again.');
            } else if (errorMessage.includes('new password and confirmation do not match')) {
              alert('The new password and confirmation password do not match!');
            } else if (errorMessage.includes('Password does not meet the required criteria')) {
              alert('The new password does not meet the required criteria. Please try a stronger password!');
            } else if (errorMessage.includes('Password is too common')) {
              alert('The new password is too common. Please choose a more unique password!');
            } else {
              alert('Error changing password: ' + errorMessage);
            }
          } else {
            alert('An unexpected error occurred. Please try again later.');
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
