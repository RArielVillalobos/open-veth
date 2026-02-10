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
  });

  it('show() should add a toast with specified type', () => {
    vi.useFakeTimers();
    service.show('oops', 'error');
    expect(service.toasts()[0].type).toBe('error');
  });

  it('error() should add an error toast', () => {
    vi.useFakeTimers();
    service.error('fail');
    expect(service.toasts()[0].type).toBe('error');
    expect(service.toasts()[0].message).toBe('fail');
  });

  it('success() should add a success toast', () => {
    vi.useFakeTimers();
    service.success('done');
    expect(service.toasts()[0].type).toBe('success');
  });

  it('info() should add an info toast', () => {
    vi.useFakeTimers();
    service.info('fyi');
    expect(service.toasts()[0].type).toBe('info');
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

  it('should auto-remove toast after 5 seconds', () => {
    vi.useFakeTimers();
    service.show('temp');
    expect(service.toasts().length).toBe(1);
    vi.advanceTimersByTime(5000);
    expect(service.toasts().length).toBe(0);
  });

  it('remove() with non-existent id should not throw', () => {
    expect(() => service.remove(999)).not.toThrow();
    expect(service.toasts()).toEqual([]);
  });

  it('should support multiple toasts', () => {
    vi.useFakeTimers();
    service.error('e1');
    service.success('s1');
    service.info('i1');
    expect(service.toasts().length).toBe(3);
  });
});
