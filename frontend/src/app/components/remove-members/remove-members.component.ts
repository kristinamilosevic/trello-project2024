import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ProjectMembersService } from '../../services/project-members/project-members.service';
import { ActivatedRoute } from '@angular/router';

@Component({
  selector: 'app-remove-members',
  standalone: true, 
  imports: [CommonModule], 
  templateUrl: './remove-members.component.html',
  styleUrls: ['./remove-members.component.css']
})
export class RemoveMembersComponent implements OnInit {
  projectId!: string; // ID projekta
  members: any[] = [];

  constructor(
    private projectMembersService: ProjectMembersService,
    private route: ActivatedRoute
  ) {}

  ngOnInit(): void {
    // Preuzmite ID projekta iz URL-a
    this.projectId = this.route.snapshot.paramMap.get('id')!;
    this.loadMembers(); // Učitajte članove
  }

  loadMembers() {
    this.projectMembersService.getProjectMembers(this.projectId).subscribe(
      (data) => {
        this.members = data; // Dodelite dobijene članove
      },
      (error) => {
        console.error('Error fetching members:', error);
      }
    );
  }

  removeMember(memberId: string) {
    this.projectMembersService.removeMember(this.projectId, memberId).subscribe(
      () => {
        this.members = this.members.filter(member => member.id !== memberId); // Uklonite člana iz lokalnog niza
        alert('Member removed successfully'); // Obavestite korisnika
        this.loadMembers(); // Ponovo učitajte članove
      },
      (error) => {
        console.error('Error removing member:', error);
        alert('Cannot remove member assigned to an in-progress task'); // Obavestite korisnika o grešci
      }
    );
  }
}
