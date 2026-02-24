import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import ChangeItem from '@/components/ChangeItem';
import { mockChange } from '@/test/helpers';

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
  } as any);
}

function renderChangeItem(
  changeOverrides?: Partial<import('@/types/change').Change>,
  handlers?: { onEdit?: (c: any) => void; onDelete?: (c: any) => void },
) {
  const change = mockChange(changeOverrides);
  return render(
    <MemoryRouter>
      <ChangeItem
        change={change}
        onEdit={handlers?.onEdit}
        onDelete={handlers?.onDelete}
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
});
