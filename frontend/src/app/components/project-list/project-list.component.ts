import { Component, OnInit } from '@angular/core';
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
  projects: Project[] = []; // Initialize empty array for projects

  constructor(private projectService: ProjectService) {} // Injecting ProjectService

  ngOnInit(): void {
    this.projectService.getProjects().subscribe(
      (data) => {
        this.projects = data.map((project) => ({
          ...project,
          expectedEndDate: new Date(project.expectedEndDate as string) // Prisilno konvertovanje u string pre kreiranja Date objekta
        }));
      },
      (error) => {
        console.error('Error fetching projects:', error);
      }
    );
  }
  
}
