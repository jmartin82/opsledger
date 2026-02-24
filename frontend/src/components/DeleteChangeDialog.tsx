import { useState } from 'react';
import { Change } from '@/types/change';
import { api } from '@/lib/api';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog';

interface DeleteChangeDialogProps {
  change: Change | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
}

const DeleteChangeDialog = ({ change, open, onOpenChange, onDeleted }: DeleteChangeDialogProps) => {
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleDelete = async () => {
    if (!change) return;
    setDeleting(true);
    setError(null);
    try {
      await api.delete(`/api/changes/${change.id}`);
      onOpenChange(false);
      onDeleted();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete change');
    } finally {
      setDeleting(false);
    }
  };

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete Change</AlertDialogTitle>
          <AlertDialogDescription>
            Are you sure you want to delete this change? This action cannot be undone.
          </AlertDialogDescription>
          {change && (
            <div className="mt-3 p-3 rounded-lg bg-secondary/50 border border-border text-sm">
              <span className="font-mono font-semibold text-foreground">{change.system}</span>
              {change.environment && <span className="text-muted-foreground"> / {change.environment}</span>}
              <p className="text-muted-foreground mt-1 text-xs leading-snug">{change.description}</p>
            </div>
          )}
          {error && <p className="text-xs text-destructive mt-2">{error}</p>}
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            disabled={deleting}
            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
          >
            {deleting ? 'Deleting...' : 'Delete'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
};

export default DeleteChangeDialog;
