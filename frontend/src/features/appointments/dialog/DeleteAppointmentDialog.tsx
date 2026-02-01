import { Loader2 } from "lucide-react";

import type { ScheduleItemModel } from "../../../api/appointments";
import { Button } from "../../../components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "../../../components/ui/dialog";
import { formatRange } from "../utils";

export type DeleteAppointmentDialogProps = {
	appointment: ScheduleItemModel | null;
	error: string | null;
	submitting: boolean;
	onOpenChange: (open: boolean) => void;
	onCancel: () => void;
	onConfirm: () => void;
};

export function DeleteAppointmentDialog({
	appointment,
	error,
	submitting,
	onOpenChange,
	onCancel,
	onConfirm,
}: DeleteAppointmentDialogProps) {
	const open = !!appointment;

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Remove appointment?</DialogTitle>
					<DialogDescription>
						{appointment
							? `${appointment.title} • ${formatRange(appointment.startTime, appointment.endTime)}`
							: null}
					</DialogDescription>
				</DialogHeader>

				{error ? (
					<div className="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{error}
					</div>
				) : null}

				<DialogFooter className="mt-4">
					<Button
						type="button"
						variant="outline"
						onClick={onCancel}
						disabled={submitting}
					>
						Cancel
					</Button>
					<Button
						type="button"
						variant="destructive"
						onClick={onConfirm}
						disabled={submitting}
					>
						{submitting ? (
							<span className="inline-flex items-center gap-2">
								<Loader2 className="h-4 w-4 animate-spin" />
								Removing
							</span>
						) : (
							"Remove"
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
