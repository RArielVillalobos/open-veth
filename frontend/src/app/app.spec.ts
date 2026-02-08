import { TestBed } from '@angular/core/testing';
import { App } from './app';
import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { provideRouter, Router } from '@angular/router';
import { Component } from '@angular/core';
import { describe, it, expect, beforeEach } from 'vitest';

// Mock Dashboard to avoid Cytoscape Canvas errors
@Component({
  selector: 'app-dashboard',
  standalone: true,
  template: '<h1>OpenVeth</h1>'
})
class MockDashboardComponent {}

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [
        provideHttpClient(),
        provideHttpClientTesting(),
        // Define a mock route specifically for this test
        provideRouter([
          { path: '', component: MockDashboardComponent }
        ])
      ]
    }).compileComponents();
  });

  it('should create the app', () => {
    const fixture = TestBed.createComponent(App);
    const app = fixture.componentInstance;
    expect(app).toBeTruthy();
  });

  it('should render title', async () => {
    const fixture = TestBed.createComponent(App);
    const router = TestBed.inject(Router);
    
    // Trigger initial navigation
    await router.navigate(['']);
    fixture.detectChanges();
    await fixture.whenStable();
    
    const compiled = fixture.nativeElement as HTMLElement;
    const h1 = compiled.querySelector('h1');
    expect(h1?.textContent).toContain('OpenVeth');
  });
});
