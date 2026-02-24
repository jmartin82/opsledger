import { http, HttpResponse } from 'msw';
import { mockUser, mockChange } from './helpers';

const API_URL = 'http://localhost:8081';

export const handlers = [
  // Auth
  http.get(`${API_URL}/api/auth/me`, () => {
    return HttpResponse.json(mockUser());
  }),

  http.get(`${API_URL}/api/auth/registration-status`, () => {
    return HttpResponse.json({ allowed: true });
  }),

  http.post(`${API_URL}/api/auth/login`, () => {
    return HttpResponse.json({ token: 'fake-jwt-token', user: mockUser() });
  }),

  http.post(`${API_URL}/api/auth/register`, () => {
    return HttpResponse.json({ token: 'fake-jwt-token', user: mockUser() });
  }),

  http.post(`${API_URL}/api/auth/logout`, () => {
    return new HttpResponse(null, { status: 200 });
  }),

  // Changes
  http.get(`${API_URL}/api/changes`, () => {
    return HttpResponse.json({
      changes: [mockChange()],
      total: 1,
      limit: 50,
      offset: 0,
    });
  }),

  http.post(`${API_URL}/api/changes`, async ({ request }) => {
    const body = await request.json() as Record<string, unknown>;
    return HttpResponse.json({
      id: '99',
      ...body,
      timestamp: body.timestamp || new Date().toISOString(),
    }, { status: 201 });
  }),

  http.put(`${API_URL}/api/changes/:id`, async ({ params, request }) => {
    const body = await request.json() as Record<string, unknown>;
    return HttpResponse.json({
      id: params.id,
      ...body,
    });
  }),

  http.delete(`${API_URL}/api/changes/:id`, () => {
    return HttpResponse.json({ message: 'Change deleted' });
  }),

  // Admin
  http.get(`${API_URL}/api/admin/users`, () => {
    return HttpResponse.json([]);
  }),

  http.get(`${API_URL}/api/admin/api-keys`, () => {
    return HttpResponse.json([]);
  }),
];
