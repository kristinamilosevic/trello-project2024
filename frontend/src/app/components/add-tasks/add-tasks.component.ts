import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup } from '@angular/forms';

@Component({
  selector: 'app-create-task',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './add-tasks.component.html',
  styleUrls: ['./add-tasks.component.css']
})
export class AddTasksComponent {
  showForm: boolean = true; // Varijabla za kontrolu vidljivosti forme
  taskForm: FormGroup; // Forma za zadatak

  constructor(private fb: FormBuilder) {
    // Inicijalizacija forme
    this.taskForm = this.fb.group({
      projectId: [0],
      title: [''],
      description: [''],
      // Dodajte ostala polja po potrebi
    });
  }

  toggleForm() {
    this.showForm = !this.showForm; // Prebaci vidljivost forme
  }

  onSubmit() {
    // Ovdje možete obraditi unos
    console.log(this.taskForm.value);
    // Zatvori formu nakon slanja
    this.showForm = false;
    this.taskForm.reset(); // Resetujte formu ako je potrebno
  }

}
