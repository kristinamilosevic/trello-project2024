import { Injectable } from '@angular/core';
import { HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})

export class TaskService {
  private apiUrl = 'http://localhost:8000/tasks';

  constructor(private http: HttpClient) {}

  // Metoda za slanje POST zahteva za kreiranje novog taska
  createTask(taskData: { projectId: string; title: string; description: string }): Observable<any> {
    const headers = new HttpHeaders({ 'Content-Type': 'application/json' });
    return this.http.post(this.apiUrl, taskData, { headers });
  }
}
