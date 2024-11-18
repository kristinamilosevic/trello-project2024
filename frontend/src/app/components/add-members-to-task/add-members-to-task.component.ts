import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { TaskService } from '../../services/task/task.service';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'app-add-members-to-task',
  templateUrl: './add-members-to-task.component.html',
  styleUrls: ['./add-members-to-task.component.css'],
  standalone: true,
  imports: [FormsModule, CommonModule] // Dodajte ovde CommonModule
})
export class AddMembersToTaskComponent implements OnInit {
  projectId: string | null = null;
  taskId: string | null = null;
  members: any[] = [];
  selectedMembers: any[] = [];
  successMessage: string | null = null;
  errorMessage: string | null = null;

  constructor(
    private taskService: TaskService,
    private route: ActivatedRoute,
    private router: Router
  ) {}

  ngOnInit(): void {
    this.projectId = this.route.snapshot.paramMap.get('projectId') || '';
    this.taskId = this.route.snapshot.paramMap.get('taskId') || '';
    if (this.projectId && this.taskId) {
      this.loadAvailableMembers();
    }
  }

  loadAvailableMembers(): void {
    if (this.projectId && this.taskId) {
        this.taskService.getAvailableMembers(this.projectId, this.taskId).subscribe(
            (data) => {
                // Filtriramo članove koji već nisu dodati ovom tasku
                this.members = data.filter(member => !member.assigned);
            },
            (error) => {
                console.error('Error fetching members:', error);
            }
        );
    }
}

  toggleMemberSelection(member: any): void {
    member.selected = !member.selected;
  }

  isAnyMemberSelected(): boolean {
    return this.members.some(member => member.selected);
  }

  addSelectedMembers(): void {
    const selectedMembers = this.members.filter(member => member.selected);

    if (!this.taskId || !this.projectId) {
      this.errorMessage = 'Invalid task or project ID';
      return;
    }

    this.taskService.addMembersToTask(this.taskId, selectedMembers).subscribe(
      () => {
        this.successMessage = 'Members added successfully!';
        setTimeout(() => {
          this.router.navigate([`/project/${this.projectId}`]);
        }, 2000);
      },
      (error) => {
        console.error('Error adding members:', error);
        this.errorMessage = 'Failed to add members';
      }
    );
  }
}
