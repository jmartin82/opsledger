import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import DeleteChangeDialog from '@/components/DeleteChangeDialog';
import { mockChange } from '@/test/helpers';

const API_URL = 'http://localhost:8081';

describe('DeleteChangeDialog', () => {
  const defaultProps = {
    change: mockChange(),
    open: true,
    onOpenChange: vi.fn(),
    onDeleted: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows change details in confirmation dialog', () => {
    render(<DeleteChangeDialog {...defaultProps} />);

    expect(screen.getByText('Delete Change')).toBeInTheDocument();
    expect(screen.getByText(/cannot be undone/)).toBeInTheDocument();
    expect(screen.getByText('api-gateway')).toBeInTheDocument();
    expect(screen.getByText('Deployed v2.3.1 with bug fixes')).toBeInTheDocument();
  });

  it('calls DELETE API on confirm', async () => {
    let deleteCalled = false;
    server.use(
      http.delete(`${API_URL}/api/changes/1`, () => {
        deleteCalled = true;
        return HttpResponse.json({ message: 'Change deleted' });
      }),
    );

    const user = userEvent.setup();
    render(<DeleteChangeDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /^delete$/i }));

    await waitFor(() => {
      expect(deleteCalled).toBe(true);
    });
  });

  it('calls onDeleted callback on success', async () => {
    const user = userEvent.setup();
    render(<DeleteChangeDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /^delete$/i }));

    await waitFor(() => {
      expect(defaultProps.onDeleted).toHaveBeenCalled();
    });
  });

  it('shows error on API failure', async () => {
    server.use(
      http.delete(`${API_URL}/api/changes/1`, () => {
        return HttpResponse.json({ error: 'Cannot delete' }, { status: 403 });
      }),
    );

    const user = userEvent.setup();
    render(<DeleteChangeDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /^delete$/i }));

    await waitFor(() => {
      expect(screen.getByText('Cannot delete')).toBeInTheDocument();
    });
  });
});
