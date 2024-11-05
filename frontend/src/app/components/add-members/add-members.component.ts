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
  projectMembers: any[] = []; // Koristimo any[] da bismo podržali _id format
  projectId: string = '672939543b45491848ab98b3'; // Zakucan ID projekta za testiranje

  constructor(private http: HttpClient) {}

  ngOnInit(): void {
    if (this.isValidObjectId(this.projectId)) {
      this.fetchProjectMembers();
    } else {
      console.error('Invalid projectId format. It should be a 24-character hex string.');
    }
  }

  isValidObjectId(id: string): boolean {
    return /^[a-f\d]{24}$/i.test(id);
  }

  fetchProjectMembers() {
    console.log("Fetching project members with projectId:", this.projectId);

    this.http.get<any[]>(`http://localhost:8080/projects/${this.projectId}/members`).subscribe(
      (projectMembers) => {
        console.log("Fetched project members:", projectMembers);

        // Pretvaramo `_id` u `id` string za svaki projektni član
        this.projectMembers = projectMembers.map(member => ({
          ...member,
          id: member._id.toString() // Preimenujemo `_id` u `id` za doslednost
        }));
        
        this.fetchUsers(); // Nakon što učitamo članove projekta, pozivamo učitavanje svih korisnika
      },
      (error) => {
        console.error('Error fetching project members:', error);
      }
    );
  }

  fetchUsers() {
    console.log('Fetching all users...');

    this.http.get<Member[]>('http://localhost:8080/users').subscribe(
      (allUsers) => {
        console.log('All users:', allUsers);

        // Proveravamo da li je svaki korisnik već član projekta
        this.members = allUsers.map(user => {
          const userId = user.id.toString(); // Osiguravamo da je `id` string
          const isSelected = this.projectMembers.some(projMember => projMember.id === userId);

          // Dodatni log za proveru vrednosti id-a
          console.log(`Comparing project member ID: ${userId} with project IDs:`, this.projectMembers.map(pm => pm.id));
          console.log(`User ${user.name} selected status:`, isSelected);
          
          return { ...user, selected: isSelected }; // Dodajemo selected svojstvo za checkiranje
        });
      },
      (error) => {
        console.error('Error fetching users:', error);
      }
    );
  }

  addSelectedMembers() {
    const newMembersToAdd = this.members
      .filter(member => member.selected && !this.isMemberAlreadyAdded(member))
      .map(member => member.id);

    if (newMembersToAdd.length === 0) {
      alert('No new members to add!');
      return;
    }

    this.http.post(`http://localhost:8080/projects/${this.projectId}/members`, newMembersToAdd).subscribe(
      () => {
        alert('Members added successfully!');
        this.fetchProjectMembers(); // Osvežavamo listu članova nakon dodavanja
      },
      (error) => {
        console.error('Error adding members:', error);
      }
    );
  }

  isMemberAlreadyAdded(member: Member): boolean {
    return this.projectMembers.some(existingMember => existingMember.id === member.id);
  }
}
