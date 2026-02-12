import { TestBed } from '@angular/core/testing';
import { ToastService } from './toast.service';
import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';

describe('ToastService', () => {
  let service: ToastService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(ToastService);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('should start with empty toasts', () => {
    expect(service.toasts()).toEqual([]);
  });

  it('show() should add a toast with default type info', () => {
    vi.useFakeTimers();
    service.show('hello');
    expect(service.toasts().length).toBe(1);
    expect(service.toasts()[0].message).toBe('hello');
    expect(service.toasts()[0].type).toBe('info');
    expect(service.toasts()[0].duration).toBe(5000);
  });

  it('show() should add a toast with specified type', () => {
    vi.useFakeTimers();
    service.show('oops', 'error');
    expect(service.toasts()[0].type).toBe('error');
    expect(service.toasts()[0].duration).toBe(8000);
  });

  it('error() should add an error toast with 8s duration', () => {
    vi.useFakeTimers();
    service.error('fail');
    expect(service.toasts()[0].type).toBe('error');
    expect(service.toasts()[0].message).toBe('fail');
    expect(service.toasts()[0].duration).toBe(8000);
  });

  it('success() should add a success toast with 3s duration', () => {
    vi.useFakeTimers();
    service.success('done');
    expect(service.toasts()[0].type).toBe('success');
    expect(service.toasts()[0].duration).toBe(3000);
  });

  it('info() should add an info toast with 5s duration', () => {
    vi.useFakeTimers();
    service.info('fyi');
    expect(service.toasts()[0].type).toBe('info');
    expect(service.toasts()[0].duration).toBe(5000);
  });

  it('remove() should remove a toast by id', () => {
    vi.useFakeTimers();
    service.show('a');
    service.show('b');
    const idToRemove = service.toasts()[0].id;
    service.remove(idToRemove);
    expect(service.toasts().length).toBe(1);
    expect(service.toasts()[0].message).toBe('b');
  });

  it('should auto-remove toast after specified duration', () => {
    vi.useFakeTimers();
    service.show('temp', 'success');
    expect(service.toasts().length).toBe(1);
    vi.advanceTimersByTime(3000);
    expect(service.toasts().length).toBe(0);
  });

  it('error toasts should last 8 seconds', () => {
    vi.useFakeTimers();
    service.error('error message');
    expect(service.toasts().length).toBe(1);
    vi.advanceTimersByTime(7000);
    expect(service.toasts().length).toBe(1); // Still there
    vi.advanceTimersByTime(1000);
    expect(service.toasts().length).toBe(0); // Gone after 8s
  });

  it('should limit toasts to maximum of 3', () => {
    vi.useFakeTimers();
    service.show('1');
    service.show('2');
    service.show('3');
    expect(service.toasts().length).toBe(3);
    
    service.show('4'); // This should remove '1'
    expect(service.toasts().length).toBe(3);
    expect(service.toasts()[0].message).toBe('2');
    expect(service.toasts()[2].message).toBe('4');
  });

  it('remove() with non-existent id should not throw', () => {
    expect(() => service.remove(999)).not.toThrow();
    expect(service.toasts()).toEqual([]);
  });

  it('clearAll() should remove all toasts', () => {
    vi.useFakeTimers();
    service.error('e1');
    service.success('s1');
    service.info('i1');
    expect(service.toasts().length).toBe(3);
    
    service.clearAll();
    expect(service.toasts().length).toBe(0);
  });

  it('should support multiple toasts up to limit', () => {
    vi.useFakeTimers();
    service.error('e1');
    service.success('s1');
    service.info('i1');
    expect(service.toasts().length).toBe(3);
    expect(service.toasts()[0].type).toBe('error');
    expect(service.toasts()[1].type).toBe('success');
    expect(service.toasts()[2].type).toBe('info');
  });
});
