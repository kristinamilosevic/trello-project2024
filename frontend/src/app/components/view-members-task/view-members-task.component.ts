import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { CommonModule } from '@angular/common';
import { TaskService } from '../../services/task/task.service';

@Component({
  selector: 'app-view-members-task',
  standalone: true,
  imports: [CommonModule], // Dodaj CommonModule ovde
  templateUrl: './view-members-task.component.html',
  styleUrls: ['./view-members-task.component.css']
})
export class ViewMembersTaskComponent implements OnInit {
  taskId: string | null = null;
  projectId: string | null = null;
  members: any[] = [];
  errorMessage: string | null = null;

  constructor(
    private route: ActivatedRoute,
    private taskService: TaskService
  ) {}

  ngOnInit(): void {
    this.projectId = this.route.snapshot.paramMap.get('projectId');
    this.taskId = this.route.snapshot.paramMap.get('taskId');
    if (this.taskId) {
      this.loadTaskMembers();
    }
  }

  loadTaskMembers(): void {
    if (this.taskId) {
      this.taskService.getTaskMembers(this.taskId).subscribe({
        next: (data) => {
          this.members = data;
        },
        error: (err) => {
          console.error('Error fetching task members:', err);
          this.errorMessage = 'Failed to fetch task members.';
        }
      });
    }
  }
}
