import { Component, OnInit } from '@angular/core';
import { HttpClient } from '@angular/common/http';
import { FormsModule } from '@angular/forms';
import { HttpClientModule } from '@angular/common/http';
import { CommonModule } from '@angular/common';
import { Member } from '../../models/member/member.model';

@Component({
  selector: 'app-add-members',
  standalone: true,
  imports: [FormsModule, HttpClientModule, CommonModule],
  templateUrl: './add-members.component.html',
  styleUrls: ['./add-members.component.css']
})
export class AddMembersComponent implements OnInit {
  members: Member[] = [];
  projectMembers: Member[] = []; // Nova lista za članove projekta
  projectId: string = '6724f9481857e7c9fab956b7'; // Hardkodiran ID projekta za sada

  constructor(private http: HttpClient) {}

  ngOnInit(): void {
    this.fetchProjectMembers();
  }

  fetchProjectMembers() {
    console.log("Fetching project members...");

    // Prvo učitavamo članove koji su već na projektu
    this.http.get<Member[]>(`http://localhost:8080/projects/6724f9481857e7c9fab956b7/members`).subscribe(
      (projectMembers) => {
        console.log("Project members:", projectMembers); // Dodaj log da vidimo vraćene članove projekta
        this.projectMembers = projectMembers; // Čuvamo članove projekta
        this.fetchUsers();
      },
      (error) => {
        console.error('Error fetching project members:', error);
      }
    );
  }

  fetchUsers() {
    console.log("Fetching all users...");

    // Zatim učitavamo sve korisnike
    this.http.get<Member[]>('http://localhost:8080/users').subscribe(
      (allUsers) => {
        console.log("All users:", allUsers); // Dodaj log za sve korisnike iz baze

        // Setujemo `selected` na `true` za članove koji su već na projektu
        this.members = allUsers.map(user => {
          const isSelected = this.projectMembers.some(projMember => projMember.id === user.id);
          console.log(`User ${user.name} selected status:`, isSelected); // Log za svaki `selected` status
          return { ...user, selected: isSelected };
        });
      },
      (error) => {
        console.error('Error fetching users:', error);
      }
    );
  }

  addSelectedMembers() {
    // Pronađemo samo one članove koji su selektovani i još nisu na projektu
    const newMembersToAdd = this.members
      .filter(member => member.selected && !this.isMemberAlreadyAdded(member))
      .map(member => member.id); // Dobijamo samo ID-jeve članova
  
    if (newMembersToAdd.length === 0) {
      alert("No new members to add!");
      return;
    }
  
    // Šaljemo niz ID-jeva umesto celih objekata
    this.http.post(`http://localhost:8080/projects/6724f9481857e7c9fab956b7/members`, newMembersToAdd).subscribe(
      () => {
        alert('Members added successfully!');
        this.fetchProjectMembers(); // Ponovo učitavamo članove nakon dodavanja
      },
      (error) => {
        console.error('Error adding members:', error);
      }
    );
  }
  

  // Pomoćna funkcija koja proverava da li je član već na projektu
  isMemberAlreadyAdded(member: Member): boolean {
    return this.projectMembers.some(existingMember => existingMember.id === member.id);
  }
}
