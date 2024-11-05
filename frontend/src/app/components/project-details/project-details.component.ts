import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { ProjectService } from '../../services/project/project.service';
import { Project } from '../../models/project/project';
import { CommonModule, DatePipe } from '@angular/common';
import { Router } from '@angular/router';

@Component({
  selector: 'app-project-details',
  standalone: true,
  imports: [CommonModule],
  providers: [DatePipe],
  templateUrl: './project-details.component.html',
  styleUrls: ['./project-details.component.css']
})
export class ProjectDetailsComponent implements OnInit {
  project: Project | null = null;

 
constructor(
  private route: ActivatedRoute,
  private projectService: ProjectService,
  private datePipe: DatePipe,
  private router: Router
) {}

  ngOnInit(): void {
    const projectId = this.route.snapshot.paramMap.get('id');
    if (projectId) {
      this.projectService.getProjectById(projectId).subscribe(
        (data) => {
          this.project = data;
        },
        (error) => {
          console.error('Error fetching project details:', error);
        }
      );
    }
  }

  goBack(): void {
    window.history.back();
  }

  // Prazne metode za dugmad
  addTask(): void {
    console.log("Add Task clicked");
  }

  viewMembers(): void {
    console.log("Members clicked");
  }

  addMember(): void {
    const projectId = this.project?.id;
    if (projectId) {
      this.router.navigate([`/project/${projectId}/add-members`]);
    }
  }
  
}
