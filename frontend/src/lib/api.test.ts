import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import { api, getToken, setToken, clearToken } from '@/lib/api';

const API_URL = 'http://localhost:8081';

describe('api', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('GET sends Authorization header when token exists', async () => {
    let capturedAuth: string | null = null;
    server.use(
      http.get(`${API_URL}/api/test`, ({ request }) => {
        capturedAuth = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );

    setToken('my-token');
    await api.get('/api/test');
    expect(capturedAuth).toBe('Bearer my-token');
  });

  it('GET sends no Authorization header when no token', async () => {
    let capturedAuth: string | null = null;
    server.use(
      http.get(`${API_URL}/api/test`, ({ request }) => {
        capturedAuth = request.headers.get('Authorization');
        return HttpResponse.json({ ok: true });
      }),
    );

    await api.get('/api/test');
    expect(capturedAuth).toBeNull();
  });

  it('POST sends JSON body', async () => {
    let capturedBody: unknown = null;
    server.use(
      http.post(`${API_URL}/api/test`, async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ ok: true });
      }),
    );

    await api.post('/api/test', { foo: 'bar' });
    expect(capturedBody).toEqual({ foo: 'bar' });
  });

  it('401 response clears token and redirects', async () => {
    server.use(
      http.get(`${API_URL}/api/test`, () => {
        return new HttpResponse(null, { status: 401 });
      }),
    );

    setToken('will-be-cleared');

    // Mock window.location.href
    const originalHref = window.location.href;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (window as any).location;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (window as any).location = { href: originalHref };

    await expect(api.get('/api/test')).rejects.toThrow('Unauthorized');
    expect(getToken()).toBeNull();
    expect(window.location.href).toBe('/login');

    // Restore
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (window as any).location = new URL(originalHref);
  });

  it('non-OK response throws with error message from body', async () => {
    server.use(
      http.get(`${API_URL}/api/test`, () => {
        return HttpResponse.json({ error: 'Something broke' }, { status: 500 });
      }),
    );

    await expect(api.get('/api/test')).rejects.toThrow('Something broke');
  });

  it('non-OK response with unparseable body throws generic message', async () => {
    server.use(
      http.get(`${API_URL}/api/test`, () => {
        return new HttpResponse('not json', { status: 500, headers: { 'Content-Type': 'text/plain' } });
      }),
    );

    await expect(api.get('/api/test')).rejects.toThrow('Request failed with status 500');
  });
});
