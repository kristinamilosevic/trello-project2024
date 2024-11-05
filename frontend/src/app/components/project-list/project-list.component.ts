import { Component, OnInit } from '@angular/core';
import { Router } from '@angular/router';
import { ProjectService } from '../../services/project/project.service';
import { Project } from '../../models/project/project';
import { CommonModule, DatePipe } from '@angular/common';

@Component({
  selector: 'app-project-list',
  standalone: true,
  imports: [CommonModule],
  providers: [DatePipe],
  templateUrl: './project-list.component.html',
  styleUrls: ['./project-list.component.css']
})
export class ProjectListComponent implements OnInit {
  projects: Project[] = [];

  constructor(private projectService: ProjectService, private router: Router) {}

  ngOnInit(): void {
    this.projectService.getProjects().subscribe(
      (data) => {
        this.projects = data.map((project) => ({
          ...project,
          expectedEndDate: new Date(project.expectedEndDate as string)
        }));
      },
      (error) => {
        console.error('Error fetching projects:', error);
      }
    );
  }

  // Navigacija na stranicu sa detaljima projekta
  openDetails(project: Project): void {
    this.router.navigate(['/project', project.id]);
  }
}
