import { useState, KeyboardEvent } from 'react';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

const AutocompleteInput = ({
  value,
  onChange,
  suggestions,
  placeholder,
  onNext,
  inputRef,
  mono = false,
}: {
  value: string;
  onChange: (v: string) => void;
  suggestions: string[];
  placeholder?: string;
  onNext?: () => void;
  inputRef?: React.RefObject<HTMLInputElement>;
  mono?: boolean;
}) => {
  const [open, setOpen] = useState(false);
  const [highlighted, setHighlighted] = useState(-1);

  const filtered = suggestions.filter(s =>
    value.length > 0 && s.toLowerCase().includes(value.toLowerCase()) && s !== value
  );

  const select = (s: string) => {
    onChange(s);
    setOpen(false);
    onNext?.();
  };

  const handleKey = (e: KeyboardEvent<HTMLInputElement>) => {
    if (!open || filtered.length === 0) {
      if (e.key === 'Tab' || e.key === 'Enter') onNext?.();
      return;
    }
    if (e.key === 'ArrowDown') { e.preventDefault(); setHighlighted(h => Math.min(h + 1, filtered.length - 1)); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setHighlighted(h => Math.max(h - 1, 0)); }
    else if ((e.key === 'Enter' || e.key === 'Tab') && highlighted >= 0) {
      e.preventDefault();
      select(filtered[highlighted]);
    } else if (e.key === 'Escape') setOpen(false);
  };

  return (
    <div className="relative">
      <Input
        ref={inputRef}
        value={value}
        onChange={e => { onChange(e.target.value); setOpen(true); setHighlighted(-1); }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        onKeyDown={handleKey}
        placeholder={placeholder}
        className={cn(
          'bg-card border-border text-sm',
          mono && 'font-mono'
        )}
        autoComplete="off"
      />
      {open && filtered.length > 0 && (
        <div className="absolute z-50 w-full mt-1 bg-popover border border-border rounded shadow-md overflow-hidden">
          {filtered.slice(0, 8).map((s, i) => (
            <button
              key={s}
              onMouseDown={() => select(s)}
              className={cn(
                'w-full text-left px-3 py-1.5 text-sm transition-colors',
                mono && 'font-mono',
                i === highlighted ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground'
              )}
            >
              {s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
};

export default AutocompleteInput;
