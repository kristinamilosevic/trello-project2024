import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule, FormBuilder, FormGroup } from '@angular/forms';
import { TaskService } from '../../services/task/task.service';
import { Router, ActivatedRoute } from '@angular/router';

@Component({
  selector: 'app-create-task',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  templateUrl: './add-tasks.component.html',
  styleUrls: ['./add-tasks.component.css']
})
export class AddTasksComponent {
  showForm: boolean = true;
  taskForm: FormGroup;
  successMessage: string = '';
  errorMessage: string = '';

  constructor(
    private fb: FormBuilder,
    private taskService: TaskService,
    private router: Router,
    private route: ActivatedRoute
  ) {
    this.taskForm = this.fb.group({
      projectId: [''],
      title: [''],
      description: [''],
      status: ['Pending']
    });

    this.route.params.subscribe(params => {
      const projectId = params['projectId'];
      if (projectId) {
        this.taskForm.get('projectId')?.setValue(projectId);
      }
    });
  }

  toggleForm() {
    this.showForm = !this.showForm;
  }

  isValidTaskData(taskData: any): boolean {
    return (
      typeof taskData.projectId === 'string' &&
      typeof taskData.title === 'string' &&
      typeof taskData.description === 'string' &&
      typeof taskData.status === 'string'
    );
  }

  onSubmit() {
    const taskData = this.taskForm.value;

    if (!taskData.projectId || !taskData.title || !taskData.description) {
      this.errorMessage = 'Please fill in all required fields.';
      this.clearMessages();
      return;
    }

    if (this.taskForm.valid) {
      taskData.projectId = taskData.projectId.toString();

      if (this.isValidTaskData(taskData)) {
        this.taskService.createTask(taskData).subscribe(
          response => {
            this.taskForm.reset();
            this.taskForm.get('status')?.setValue('Pending');
<<<<<<< HEAD
            alert('Task successfully created!');
            this.router.navigate([`/project/${taskData.projectId}`]);
=======
            this.successMessage = 'Task successfully created!';
            this.clearMessages();
            this.router.navigate(['/task-list']);
>>>>>>> 9a6567d737c3fc473a9bba62398a6f3a6853154a
          },
          error => {
            console.error('Error while creating task.', error);
            this.errorMessage = 'Error while creating task.';
            this.clearMessages();
          }
        );
      } else {
        this.errorMessage = 'Invalid task data format.';
        this.clearMessages();
      }
    } else {
      this.errorMessage = 'Form is not valid.';
      this.clearMessages();
    }
  }

  clearMessages() {
    setTimeout(() => {
      this.successMessage = '';
      this.errorMessage = '';
    }, 3000);
  }
}
