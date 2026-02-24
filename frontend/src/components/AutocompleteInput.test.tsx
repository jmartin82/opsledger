import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AutocompleteInput from '@/components/AutocompleteInput';

const suggestions = ['api-gateway', 'auth-service', 'billing-service', 'frontend'];

function renderAutocomplete(overrides?: Partial<Parameters<typeof AutocompleteInput>[0]>) {
  const onChange = vi.fn();
  const onNext = vi.fn();
  const defaultProps = {
    value: '',
    onChange,
    suggestions,
    placeholder: 'Type here',
    onNext,
    ...overrides,
  };
  const result = render(<AutocompleteInput {...defaultProps} />);
  return { ...result, onChange, onNext };
}

describe('AutocompleteInput', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows suggestions when typing matching text', async () => {
    // We need to control value externally since it's a controlled component
    let currentValue = '';
    const onChange = vi.fn((v: string) => { currentValue = v; });

    const { rerender } = render(
      <AutocompleteInput
        value={currentValue}
        onChange={onChange}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    const input = screen.getByPlaceholderText('Type here');
    const user = userEvent.setup();

    await user.click(input);
    await user.type(input, 'a');

    // Rerender with the new value so filtered suggestions appear
    rerender(
      <AutocompleteInput
        value="a"
        onChange={onChange}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    // "api-gateway" and "auth-service" both contain "a"
    expect(screen.getByText('api-gateway')).toBeInTheDocument();
    expect(screen.getByText('auth-service')).toBeInTheDocument();
  });

  it('hides suggestions when input is empty', () => {
    render(
      <AutocompleteInput
        value=""
        onChange={vi.fn()}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    // No dropdown items should be visible
    expect(screen.queryByText('api-gateway')).not.toBeInTheDocument();
  });

  it('ArrowDown/ArrowUp navigates suggestions', async () => {
    const onChange = vi.fn();
    render(
      <AutocompleteInput
        value="api"
        onChange={onChange}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    const input = screen.getByPlaceholderText('Type here');
    const user = userEvent.setup();

    await user.click(input);

    // "api-gateway" should be showing
    expect(screen.getByText('api-gateway')).toBeInTheDocument();

    // Navigate down
    await user.keyboard('{ArrowDown}');

    // The first item should be highlighted (has bg-accent class)
    const item = screen.getByText('api-gateway');
    expect(item.className).toContain('bg-accent');
  });

  it('Enter selects highlighted suggestion', async () => {
    const onChange = vi.fn();
    render(
      <AutocompleteInput
        value="api"
        onChange={onChange}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    const input = screen.getByPlaceholderText('Type here');
    const user = userEvent.setup();

    await user.click(input);
    await user.keyboard('{ArrowDown}');
    await user.keyboard('{Enter}');

    expect(onChange).toHaveBeenCalledWith('api-gateway');
  });

  it('Escape closes suggestion list', async () => {
    render(
      <AutocompleteInput
        value="api"
        onChange={vi.fn()}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    const input = screen.getByPlaceholderText('Type here');
    const user = userEvent.setup();

    await user.click(input);
    expect(screen.getByText('api-gateway')).toBeInTheDocument();

    await user.keyboard('{Escape}');

    expect(screen.queryByText('api-gateway')).not.toBeInTheDocument();
  });

  it('click on suggestion fills input', async () => {
    const onChange = vi.fn();
    render(
      <AutocompleteInput
        value="api"
        onChange={onChange}
        suggestions={suggestions}
        placeholder="Type here"
      />,
    );

    const user = userEvent.setup();
    const input = screen.getByPlaceholderText('Type here');
    await user.click(input);

    await user.click(screen.getByText('api-gateway'));

    expect(onChange).toHaveBeenCalledWith('api-gateway');
  });
});
