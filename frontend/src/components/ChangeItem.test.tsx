import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/server';
import ChangeItem from '@/components/ChangeItem';
import { mockChange } from '@/test/helpers';

const API_URL = 'http://localhost:8081';

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

import { useAuth } from '@/contexts/AuthContext';
const mockUseAuth = vi.mocked(useAuth);

function setRole(role: 'admin' | 'editor' | 'viewer') {
  mockUseAuth.mockReturnValue({
    can: (action: string) => {
      if (action === 'edit_changes') return role === 'admin' || role === 'editor';
      return true;
    },
  } as ReturnType<typeof useAuth>);
}

function renderChangeItem(
  changeOverrides?: Partial<import('@/types/change').Change>,
  handlers?: {
    onEdit?: (c: import('@/types/change').Change) => void;
    onDelete?: (c: import('@/types/change').Change) => void;
    onConfirm?: (c: import('@/types/change').Change) => void;
  },
) {
  const change = mockChange(changeOverrides);
  return render(
    <MemoryRouter>
      <ChangeItem
        change={change}
        onEdit={handlers?.onEdit}
        onDelete={handlers?.onDelete}
        onConfirm={handlers?.onConfirm}
      />
    </MemoryRouter>,
  );
}

describe('ChangeItem', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setRole('editor');
  });

  it('renders change data', () => {
    renderChangeItem();

    expect(screen.getByText('api-gateway')).toBeInTheDocument();
    expect(screen.getByText('Deployment')).toBeInTheDocument();
    expect(screen.getByText('Deployed v2.3.1 with bug fixes')).toBeInTheDocument();
    expect(screen.getByText('alice.martin')).toBeInTheDocument();
  });

  it('shows environment badge when present', () => {
    renderChangeItem({ environment: 'production' });
    expect(screen.getByText('production')).toBeInTheDocument();
  });

  it('hides environment when absent', () => {
    renderChangeItem({ environment: undefined });
    expect(screen.queryByText('production')).not.toBeInTheDocument();
    expect(screen.queryByText('staging')).not.toBeInTheDocument();
  });

  it('shows edit/delete buttons for editor role', () => {
    setRole('editor');
    const onEdit = vi.fn();
    const onDelete = vi.fn();
    renderChangeItem({}, { onEdit, onDelete });

    expect(screen.getByTitle('Edit change')).toBeInTheDocument();
    expect(screen.getByTitle('Delete change')).toBeInTheDocument();
  });

  it('hides edit/delete buttons for viewer role', () => {
    setRole('viewer');
    const onEdit = vi.fn();
    const onDelete = vi.fn();
    renderChangeItem({}, { onEdit, onDelete });

    expect(screen.queryByTitle('Edit change')).not.toBeInTheDocument();
    expect(screen.queryByTitle('Delete change')).not.toBeInTheDocument();
  });

  it('shows Scheduled badge for scheduled change', () => {
    renderChangeItem({ status: 'scheduled', timestamp: '2099-12-01T10:00:00Z' });
    expect(screen.getByText('Scheduled')).toBeInTheDocument();
  });

  it('shows Overdue badge for past-dated scheduled change', () => {
    renderChangeItem({ status: 'scheduled', timestamp: '2020-01-01T00:00:00Z' });
    expect(screen.getByText('Overdue')).toBeInTheDocument();
  });

  it('shows Mark as Done button for scheduled change with editor role', () => {
    setRole('editor');
    renderChangeItem(
      { status: 'scheduled', timestamp: '2099-12-01T10:00:00Z' },
      { onConfirm: vi.fn() },
    );
    expect(screen.getByTitle('Mark as done')).toBeInTheDocument();
  });

  it('calls confirm API and invokes onConfirm callback when Mark as Done is clicked', async () => {
    const onConfirm = vi.fn();
    setRole('editor');

    server.use(
      http.patch(`${API_URL}/api/changes/:id/confirm`, ({ params }) => {
        return HttpResponse.json(mockChange({ id: params.id as string, status: 'executed' }));
      }),
    );

    const user = userEvent.setup();
    renderChangeItem(
      { id: '42', status: 'scheduled', timestamp: '2099-12-01T10:00:00Z' },
      { onConfirm },
    );

    await user.click(screen.getByTitle('Mark as done'));

    await waitFor(() => {
      expect(onConfirm).toHaveBeenCalled();
    });
  });
});
