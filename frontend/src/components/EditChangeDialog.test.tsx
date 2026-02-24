import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import EditChangeDialog from '@/components/EditChangeDialog';
import { mockChange } from '@/test/helpers';

const API_URL = 'http://localhost:8081';

// EditChangeDialog uses AutocompleteInput which doesn't need auth,
// but Layout (not used here) would. Dialog renders standalone.

describe('EditChangeDialog', () => {
  const defaultProps = {
    change: mockChange(),
    open: true,
    onOpenChange: vi.fn(),
    onSaved: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('pre-populates form fields from change prop', () => {
    render(
      <MemoryRouter>
        <EditChangeDialog {...defaultProps} />
      </MemoryRouter>,
    );

    expect(screen.getByText('Edit Change')).toBeInTheDocument();
    // System input should have the value
    expect(screen.getByPlaceholderText('e.g. api-gateway')).toHaveValue('api-gateway');
    // Description textarea
    expect(screen.getByPlaceholderText(/describe what changed/i)).toHaveValue('Deployed v2.3.1 with bug fixes');
  });

  it('validation prevents empty required fields', async () => {
    const user = userEvent.setup();
    const change = mockChange({ system: '', type: 'deployment', description: '' });
    render(
      <MemoryRouter>
        <EditChangeDialog {...defaultProps} change={change} />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: /save changes/i }));

    expect(screen.getByText('System is required')).toBeInTheDocument();
    expect(screen.getByText('Description is required')).toBeInTheDocument();
  });

  it('successful edit calls PUT with correct payload', async () => {
    let capturedBody: unknown = null;
    server.use(
      http.put(`${API_URL}/api/changes/1`, async ({ request }) => {
        capturedBody = await request.json();
        return HttpResponse.json({ id: '1', ...capturedBody });
      }),
    );

    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <EditChangeDialog {...defaultProps} />
      </MemoryRouter>,
    );

    // Modify description
    const descField = screen.getByPlaceholderText(/describe what changed/i);
    await user.clear(descField);
    await user.type(descField, 'Updated deployment');

    await user.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(capturedBody).toBeTruthy();
      expect(capturedBody.system).toBe('api-gateway');
      expect(capturedBody.description).toBe('Updated deployment');
      expect(capturedBody.type).toBe('deployment');
    });
  });

  it('calls onSaved and closes dialog on success', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <EditChangeDialog {...defaultProps} />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(defaultProps.onSaved).toHaveBeenCalled();
      expect(defaultProps.onOpenChange).toHaveBeenCalledWith(false);
    });
  });

  it('shows error on API failure', async () => {
    server.use(
      http.put(`${API_URL}/api/changes/1`, () => {
        return HttpResponse.json({ error: 'Validation failed' }, { status: 400 });
      }),
    );

    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <EditChangeDialog {...defaultProps} />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: /save changes/i }));

    await waitFor(() => {
      expect(screen.getByText('Validation failed')).toBeInTheDocument();
    });
  });
});
