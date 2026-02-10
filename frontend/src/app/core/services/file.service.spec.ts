import { TestBed } from '@angular/core/testing';
import { FileService } from './file.service';
import { describe, it, expect, beforeEach, vi } from 'vitest';

describe('FileService', () => {
  let service: FileService;

  beforeEach(() => {
    TestBed.configureTestingModule({});
    service = TestBed.inject(FileService);
  });

  describe('downloadBlob', () => {
    it('should create object URL, click anchor, and revoke URL', () => {
      const blob = new Blob(['test'], { type: 'text/plain' });
      const fakeUrl = 'blob:http://localhost/fake';

      const createSpy = vi.spyOn(window.URL, 'createObjectURL').mockReturnValue(fakeUrl);
      const revokeSpy = vi.spyOn(window.URL, 'revokeObjectURL').mockImplementation(() => {});
      const clickSpy = vi.fn();
      vi.spyOn(document, 'createElement').mockReturnValue({
        set href(_: string) {},
        set download(_: string) {},
        click: clickSpy,
      } as any);

      service.downloadBlob(blob, 'test.yaml');

      expect(createSpy).toHaveBeenCalledWith(blob);
      expect(clickSpy).toHaveBeenCalled();
      expect(revokeSpy).toHaveBeenCalledWith(fakeUrl);
    });

    it('should set the download filename', () => {
      const blob = new Blob(['data']);
      let downloadAttr = '';

      vi.spyOn(window.URL, 'createObjectURL').mockReturnValue('blob:fake');
      vi.spyOn(window.URL, 'revokeObjectURL').mockImplementation(() => {});
      vi.spyOn(document, 'createElement').mockReturnValue({
        set href(_: string) {},
        set download(val: string) { downloadAttr = val; },
        click: vi.fn(),
      } as any);

      service.downloadBlob(blob, 'topology.yaml');
      expect(downloadAttr).toBe('topology.yaml');
    });
  });

  describe('readAsText', () => {
    it('should resolve with file content', async () => {
      const file = new File(['hello world'], 'test.txt', { type: 'text/plain' });
      const result = await service.readAsText(file);
      expect(result).toBe('hello world');
    });

    it('should reject on read error', async () => {
      const file = new File([''], 'test.txt');

      // Mock FileReader with a class to allow `new` usage
      let capturedReader: any;
      class MockFileReader {
        onload: any = null;
        onerror: any = null;
        readAsText = vi.fn();
        constructor() { capturedReader = this; }
      }
      const OriginalFileReader = globalThis.FileReader;
      vi.stubGlobal('FileReader', MockFileReader);

      const promise = service.readAsText(file);

      // Trigger error callback set by the service
      capturedReader.onerror(new Event('error'));

      await expect(promise).rejects.toThrow('Error reading file');

      vi.stubGlobal('FileReader', OriginalFileReader);
    });
  });
});
