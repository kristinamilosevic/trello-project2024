import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { HttpClient, HttpClientModule } from '@angular/common/http';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
  selector: 'app-verify-code',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule, HttpClientModule],
  templateUrl: './verify-code.component.html',
  styleUrls: ['./verify-code.component.css']
})
export class VerifyCodeComponent implements OnInit {
  verifyCodeForm: FormGroup;
  username: string = '';

  constructor(private fb: FormBuilder, private http: HttpClient, private route: ActivatedRoute, private router: Router) {
    this.verifyCodeForm = this.fb.group({
      code: ['', [Validators.required, Validators.minLength(6), Validators.maxLength(6)]]
    });
  }

  ngOnInit(): void {
    // Uzimanje username-a iz query parametara
    this.route.queryParams.subscribe(params => {
      this.username = params['username'];
    });
  }

  onSubmit() {
    if (this.verifyCodeForm.valid) {
      // Priprema podataka za slanje - koristimo username iz query parametara
      const data = {
        username: this.username,
        code: this.verifyCodeForm.value.code
      };

      this.http.post('http://localhost:8080/verify-code', data, { responseType: 'text' }).subscribe({
        next: (response) => {
          console.log('Response from server:', response);
          alert('Verification successful. You can now log in.');
          this.verifyCodeForm.reset();
          // Preusmeravanje na login stranicu nakon uspešne verifikacije
          this.router.navigate(['/login']);
        },
        error: (error) => {
          console.error('Error during verification:', error);
          alert('Verification failed. Please try again.');
        },
      });
    } else {
      alert('Please fill out the form correctly.');
    }
  }
}
