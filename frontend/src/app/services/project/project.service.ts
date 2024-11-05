import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';
import { Project } from '../../models/project/project';




@Injectable({
  providedIn: 'root'
})
export class ProjectService {
  private apiUrl = 'http://localhost:8080/projects';

  constructor(private http: HttpClient) {}

  createProject(projectData: { name: string; expectedEndDate: string; minMembers: number; maxMembers: number }): Observable<any> {
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Manager-ID': '507f191e810c19729de860ea' // Obezbedi da se ovaj ID dodaje kao header, a ne u URL
    });

    // Pravilno šaljemo `headers` parametar u POST metodu
    return this.http.post(this.apiUrl, projectData, { headers });
  }

  getProjects(): Observable<Project[]> {
    return this.http.get<Project[]>(this.apiUrl); // GET zahtev za dobijanje liste projekata
  }
}
