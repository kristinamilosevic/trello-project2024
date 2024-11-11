export interface User {
    id?: string;
    name: string;
    lastName: string;
    username: string;
    email: string;
    password: string;
    role: string;
    isActive: boolean;
  }
  
  export interface LoginRequest {
    email: string;
    password: string;
  }
  
  export interface LoginResponse {
    token: string;
    email: string;
    role: string;
  }
  