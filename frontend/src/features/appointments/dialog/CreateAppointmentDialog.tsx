import { addDays, addMinutes, format } from "date-fns";
import { CalendarDays, Loader2 } from "lucide-react";

import { Button } from "../../../components/ui/button";
import { Calendar } from "../../../components/ui/calendar";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "../../../components/ui/dialog";
import { Input } from "../../../components/ui/input";
import { Label } from "../../../components/ui/label";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "../../../components/ui/popover";
import { Textarea } from "../../../components/ui/textarea";
import { cn } from "../../../lib/utils";
import {
	formatRange,
	fromTimeInputValue,
	minutesBetween,
	recurrenceHorizonEnd,
	toTimeInputValue,
	weeklyMaxOccurrencesInRange,
	type UiError,
} from "../utils";

export type CreateAppointmentDialogProps = {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	date: Date;
	dayStart: Date;
	start: Date | null;
	end: Date | null;
	title: string;
	notes: string;
	error: UiError | null;
	submitting: boolean;
	recurrenceMode: "once" | "weekly";
	weeklyInterval: number;
	weeklyWeekdays: Array<1 | 2 | 3 | 4 | 5 | 6 | 7>;
	weeklyEndsMode: "until" | "count";
	weeklyUntil: Date | null;
	weeklyCount: number;
	timeZone: string;
	onTitleChange: (value: string) => void;
	onNotesChange: (value: string) => void;
	onStartChange: (nextStart: Date, nextEnd: Date) => void;
	onEndChange: (nextEnd: Date) => void;
	onRecurrenceModeChange: (mode: "once" | "weekly") => void;
	onWeeklyIntervalChange: (interval: number) => void;
	onWeeklyWeekdayToggle: (weekday: 1 | 2 | 3 | 4 | 5 | 6 | 7) => void;
	onWeeklyEndsModeChange: (mode: "until" | "count") => void;
	onWeeklyUntilChange: (until: Date | null) => void;
	onWeeklyCountChange: (count: number) => void;
	onCancel: () => void;
	onSubmit: () => void;
};

export function CreateAppointmentDialog({
	open,
	onOpenChange,
	date,
	dayStart,
	start,
	end,
	title,
	notes,
	error,
	submitting,
	recurrenceMode,
	weeklyInterval,
	weeklyWeekdays,
	weeklyEndsMode,
	weeklyUntil,
	weeklyCount,
	timeZone,
	onTitleChange,
	onNotesChange,
	onStartChange,
	onEndChange,
	onRecurrenceModeChange,
	onWeeklyIntervalChange,
	onWeeklyWeekdayToggle,
	onWeeklyEndsModeChange,
	onWeeklyUntilChange,
	onWeeklyCountChange,
	onCancel,
	onSubmit,
}: CreateAppointmentDialogProps) {
	const description =
		start && end
			? `${format(date, "EEE, MMM d")} • ${formatRange(start, end)}`
			: "Pick a time range.";

	const weekdayButtons: Array<{
		value: 1 | 2 | 3 | 4 | 5 | 6 | 7;
		label: string;
		aria: string;
	}> = [
		{ value: 1, label: "M", aria: "Monday" },
		{ value: 2, label: "T", aria: "Tuesday" },
		{ value: 3, label: "W", aria: "Wednesday" },
		{ value: 4, label: "T", aria: "Thursday" },
		{ value: 5, label: "F", aria: "Friday" },
		{ value: 6, label: "S", aria: "Saturday" },
		{ value: 7, label: "S", aria: "Sunday" },
	];

	const recurrenceStart = start ?? dayStart;
	const untilValue =
		weeklyUntil ?? (start ? addDays(start, 28) : addDays(dayStart, 28));
	const horizonEnd = recurrenceHorizonEnd(recurrenceStart);
	const maxCount = weeklyMaxOccurrencesInRange({
		start: recurrenceStart,
		interval: weeklyInterval,
		weekdays: weeklyWeekdays,
		endInclusive: horizonEnd,
	});

	const recurrenceSummary =
		recurrenceMode === "weekly"
			? `Weekly • ${weeklyWeekdays.length ? weeklyWeekdays.length : 1} day(s) • ${weeklyEndsMode === "count" ? `for ${weeklyCount} occurrences` : `until ${format(untilValue, "MMM d")}`}`
			: "Does not repeat";

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>New appointment</DialogTitle>
					<DialogDescription>{description}</DialogDescription>
				</DialogHeader>

				<div className="mt-3 grid gap-3">
					<div className="grid gap-1.5">
						<Label htmlFor="title">Title</Label>
						<Input
							id="title"
							value={title}
							onChange={(e) => onTitleChange(e.target.value)}
							placeholder="Design review"
							autoFocus
						/>
					</div>

					<div className="grid grid-cols-2 gap-3">
						<div className="grid gap-1.5">
							<Label htmlFor="start">Start</Label>
							<Input
								id="start"
								type="time"
								step={900}
								value={start ? toTimeInputValue(start) : ""}
								onChange={(e) => {
									if (!start || !end) return;
									const nextStart = fromTimeInputValue(
										dayStart,
										e.target.value,
									);
									const duration = minutesBetween(start, end);
									onStartChange(
										nextStart,
										addMinutes(
											nextStart,
											Math.max(15, duration),
										),
									);
								}}
							/>
						</div>
						<div className="grid gap-1.5">
							<Label htmlFor="end">End</Label>
							<Input
								id="end"
								type="time"
								step={900}
								value={end ? toTimeInputValue(end) : ""}
								onChange={(e) => {
									if (!end) return;
									onEndChange(
										fromTimeInputValue(
											dayStart,
											e.target.value,
										),
									);
								}}
							/>
						</div>
					</div>

					<div className="grid gap-1.5">
						<Label>Repeats</Label>
						<div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-muted/20 p-2">
							<div className="min-w-0">
								<div className="text-xs font-medium text-muted-foreground">
									{recurrenceSummary}
								</div>
							</div>
							<div className="flex shrink-0 items-center gap-1 rounded-md border border-border bg-background p-1 shadow-sm">
								<Button
									type="button"
									variant={
										recurrenceMode === "once"
											? "secondary"
											: "ghost"
									}
									size="sm"
									onClick={() =>
										onRecurrenceModeChange("once")
									}
								>
									Once
								</Button>
								<Button
									type="button"
									variant={
										recurrenceMode === "weekly"
											? "secondary"
											: "ghost"
									}
									size="sm"
									onClick={() =>
										onRecurrenceModeChange("weekly")
									}
								>
									Weekly
								</Button>
							</div>
						</div>
					</div>

					{recurrenceMode === "weekly" ? (
						<div className="grid gap-3 rounded-xl border border-border bg-card p-3 shadow-sm ring-1 ring-black/5">
							<div className="grid grid-cols-[1fr_1fr] gap-3">
								<div className="grid gap-1.5">
									<Label htmlFor="weekly-interval">
										Every
									</Label>
									<div className="flex items-center gap-2">
										<Input
											id="weekly-interval"
											type="number"
											min={1}
											max={52}
											value={String(weeklyInterval)}
											onChange={(e) =>
												onWeeklyIntervalChange(
													Number(e.target.value),
												)
											}
											className="h-9"
										/>
										<div className="text-xs text-muted-foreground">
											week(s)
										</div>
									</div>
								</div>

								<div className="grid gap-1.5">
									<Label>Time zone</Label>
									<div className="h-9 truncate rounded-md border border-border bg-muted/20 px-3 py-2 text-xs font-medium text-muted-foreground">
										{timeZone}
									</div>
								</div>
							</div>

							<div className="grid gap-1.5">
								<Label>On</Label>
								<div className="flex flex-wrap gap-2">
									{weekdayButtons.map((b) => {
										const selected =
											weeklyWeekdays.includes(b.value);
										return (
											<Button
												key={b.value}
												type="button"
												variant="outline"
												size="sm"
												aria-pressed={selected}
												aria-label={b.aria}
												onClick={() =>
													onWeeklyWeekdayToggle(
														b.value,
													)
												}
												className={cn(
													"h-8 w-8 p-0",
													selected
														? "border-primary/40 bg-primary text-primary-foreground hover:bg-primary/90"
														: "bg-background",
												)}
											>
												{b.label}
											</Button>
										);
									})}
								</div>
							</div>

							<div className="grid gap-1.5">
								<Label>Ends</Label>
								<div className="grid gap-2">
									<div className="text-xs text-muted-foreground">
										Conflict checks are limited to 180 days.
									</div>
									<div className="flex items-center gap-1 rounded-md border border-border bg-background p-1 shadow-sm">
										<Button
											type="button"
											variant={
												weeklyEndsMode === "until"
													? "secondary"
													: "ghost"
											}
											size="sm"
											onClick={() =>
												onWeeklyEndsModeChange("until")
											}
										>
											Until
										</Button>
										<Button
											type="button"
											variant={
												weeklyEndsMode === "count"
													? "secondary"
													: "ghost"
											}
											size="sm"
											onClick={() =>
												onWeeklyEndsModeChange("count")
											}
										>
											After
										</Button>
									</div>

									{weeklyEndsMode === "count" ? (
										<div className="grid gap-1">
											<div className="flex items-center gap-2">
												<Input
													type="number"
													min={1}
													max={Math.max(1, maxCount)}
													value={String(weeklyCount)}
													onChange={(e) =>
														onWeeklyCountChange(
															Number(
																e.target.value,
															),
														)
													}
												/>
												<div className="text-xs text-muted-foreground">
													occurrences
												</div>
											</div>
											<div className="text-xs text-muted-foreground">
												Max {Math.max(1, maxCount)}{" "}
												within 180 days
											</div>
										</div>
									) : (
										<div className="flex items-center gap-2">
											<Popover>
												<PopoverTrigger asChild>
													<Button
														type="button"
														variant="outline"
														className="w-56 justify-between"
													>
														<span className="truncate">
															{format(
																untilValue,
																"EEE, MMM d",
															)}
														</span>
														<CalendarDays className="h-4 w-4 text-muted-foreground" />
													</Button>
												</PopoverTrigger>
												<PopoverContent
													align="start"
													className="p-2"
												>
													<Calendar
														mode="single"
														selected={untilValue}
														disabled={{
															before: recurrenceStart,
															after: horizonEnd,
														}}
														onSelect={(d) => {
															if (!d) return;
															const next =
																new Date(d);
															next.setHours(
																recurrenceStart.getHours(),
																recurrenceStart.getMinutes(),
																recurrenceStart.getSeconds(),
																recurrenceStart.getMilliseconds(),
															);
															const clamped =
																next.getTime() <
																recurrenceStart.getTime()
																	? recurrenceStart
																	: next.getTime() >
																		  horizonEnd.getTime()
																		? horizonEnd
																		: next;
															onWeeklyUntilChange(
																clamped,
															);
														}}
														autoFocus
													/>
												</PopoverContent>
											</Popover>
											<div className="text-xs text-muted-foreground">
												last start
											</div>
										</div>
									)}
								</div>
							</div>
						</div>
					) : null}

					<div className="grid gap-1.5">
						<Label htmlFor="notes">Notes</Label>
						<Textarea
							id="notes"
							value={notes}
							onChange={(e) => onNotesChange(e.target.value)}
							placeholder="Optional details…"
						/>
					</div>

					{error ? (
						<div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
							<div className="font-medium">{error.title}</div>
							<div className="mt-1 text-destructive/90">
								{error.message}
							</div>
						</div>
					) : null}
				</div>

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
						onClick={onSubmit}
						disabled={submitting}
					>
						{submitting ? (
							<span className="inline-flex items-center gap-2">
								<Loader2 className="h-4 w-4 animate-spin" />
								Creating
							</span>
						) : (
							"Create"
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
