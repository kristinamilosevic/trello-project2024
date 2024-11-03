import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class ProjectService {
  private apiUrl = 'http://localhost:8001/projects';

  constructor(private http: HttpClient) {}

  createProject(projectData: { name: string; expectedEndDate: string; minMembers: number; maxMembers: number }): Observable<any> {
    const headers = new HttpHeaders({
      'Content-Type': 'application/json',
      'Manager-ID': '507f191e810c19729de860ea' // Obezbedi da se ovaj ID dodaje kao header, a ne u URL
    });

    // Pravilno šaljemo `headers` parametar u POST metodu
    return this.http.post(this.apiUrl, projectData, { headers });
  }
}
