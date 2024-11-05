import { Component, OnInit } from '@angular/core';
import { ProjectMembersService } from '../../services/project-members/project-members.service';
import { Member } from '../../models/member/member.model';
import { FormsModule } from '@angular/forms';
import { HttpClientModule } from '@angular/common/http';
import { CommonModule } from '@angular/common';
import { ActivatedRoute } from '@angular/router';


@Component({
  selector: 'app-add-members',
  standalone: true,
  imports: [FormsModule, HttpClientModule, CommonModule],
  templateUrl: './add-members.component.html',
  styleUrls: ['./add-members.component.css']
})
export class AddMembersComponent implements OnInit {
  members: Member[] = [];
  projectMembers: Member[] = [];
  projectId: string = '';
  errorMessage: string = '';

  constructor(private projectMembersService: ProjectMembersService, private route: ActivatedRoute) {}

  ngOnInit(): void {
    this.projectId = this.route.snapshot.paramMap.get('id') || ''; // Preuzimamo ID projekta iz parametra rute

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
    this.projectMembersService.getProjectMembers(this.projectId).subscribe(
      (projectMembers) => {
        this.projectMembers = projectMembers.map(member => ({
          ...member,
          id: (member as any)._id.toString()
        }));
        this.fetchUsers();
      },
      (error) => {
        console.error('Error fetching project members:', error);
      }
    );
  }

  fetchUsers() {
    this.projectMembersService.getAllUsers().subscribe(
      (allUsers) => {
        this.members = allUsers.map(user => {
          const userId = user.id.toString();
          const isSelected = this.projectMembers.some(projMember => projMember.id === userId);
          return { ...user, selected: isSelected };
        });
      },
      (error) => {
        console.error('Error fetching users:', error);
      }
    );
  }

  addSelectedMembers() {
    this.errorMessage = ''; // Reset error message
  
    const newMembersToAdd = this.members
      .filter(member => member.selected && !this.isMemberAlreadyAdded(member))
      .map(member => member.id);
  
    if (newMembersToAdd.length === 0) {
      // Ako nije izabran nijedan novi član, postavi poruku o grešci i prekini funkciju
      this.errorMessage = 'No new members selected for addition.';
      return;
    }
  
    const currentMemberCount = this.projectMembers.length;
    const maxMembersAllowed = 10; // Zamenite stvarnom maksimalnom vrednošću sa backenda
    const minMembersAllowed = 2;  // Zamenite stvarnom minimalnom vrednošću sa backenda
  
    if (currentMemberCount + newMembersToAdd.length > maxMembersAllowed) {
      this.errorMessage = 'You cannot add more members than the maximum allowed.';
      return;
    }

    if (currentMemberCount + newMembersToAdd.length < minMembersAllowed) {
      this.errorMessage = 'You cannot have fewer members than the minimum required.';
      return;
    }
  
    this.projectMembersService.addMembers(this.projectId, newMembersToAdd).subscribe(
      () => {
        this.errorMessage = ''; // Očisti poruku o grešci kada je dodavanje uspešno
        alert('Members added successfully!');
        this.fetchProjectMembers();
      },
      (error) => {
        console.error('Error adding members:', error);
        if (error.status === 400) {
          // Provera za specifične poruke grešaka koje vraća backend
          const errorText = error.error || error.message || '';
          if (errorText.includes('the number of members cannot be less than the minimum required for the project')) {
            this.errorMessage = 'The number of members cannot be less than the minimum required for the project.';
          } else if (errorText.includes('maximum number of members reached for the project')) {
            this.errorMessage = 'The maximum number of members on the project has been reached!';
          } else {
            this.errorMessage = 'An error occurred while adding members.';
          }
        } else {
          this.errorMessage = 'An unexpected error occurred while adding members.';
        }
      }
    );
  }

  

  isMemberAlreadyAdded(member: Member): boolean {
    return this.projectMembers.some(existingMember => existingMember.id === member.id);
  }
}
