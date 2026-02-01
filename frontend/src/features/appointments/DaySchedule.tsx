import { format } from "date-fns";
import {
	CalendarDays,
	ChevronLeft,
	ChevronRight,
	Loader2,
	Plus,
} from "lucide-react";

import { Button } from "../../components/ui/button";
import { Calendar } from "../../components/ui/calendar";
import { Input } from "../../components/ui/input";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "../../components/ui/popover";
import { Separator } from "../../components/ui/separator";
import { CreateAppointmentDialog } from "./dialog/CreateAppointmentDialog";
import { DeleteAppointmentDialog } from "./dialog/DeleteAppointmentDialog";
import { ScheduleGrid } from "./component/ScheduleGrid";
import { useDaySchedule } from "./hooks";

export function DaySchedule() {
	const schedule = useDaySchedule();

	return (
		<div className="mx-auto w-full max-w-6xl px-5 py-6">
			<div className="flex flex-col gap-5">
				<div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
					<div className="space-y-1">
						<div className="flex items-center gap-2">
							<div className="grid h-9 w-9 place-items-center rounded-lg bg-card shadow-sm ring-1 ring-border">
								<CalendarDays className="h-5 w-5 text-foreground" />
							</div>
							<div>
								<div className="text-sm font-semibold tracking-tight">
									Appointment Schedule
								</div>
								<div className="text-xs text-muted-foreground">
									Click and drag to book time. Click an
									appointment to remove it.
								</div>
							</div>
						</div>
					</div>

					<div className="flex flex-col gap-2 sm:flex-row sm:items-center">
						<div className="flex items-center gap-2 rounded-lg border border-border bg-card px-3 py-2 shadow-sm">
							<div className="text-xs font-medium text-muted-foreground">
								User
							</div>
							<Input
								value={schedule.userId}
								onChange={(e) =>
									schedule.setUserId(e.target.value)
								}
								className="h-8 w-35 border-none bg-transparent px-2 shadow-none focus-visible:ring-0"
								placeholder="user_id"
								aria-label="user_id"
							/>
						</div>

						<div className="flex items-center gap-2">
							<Button
								variant="outline"
								size="icon"
								onClick={schedule.goPrevDay}
								aria-label="Previous day"
							>
								<ChevronLeft className="h-4 w-4" />
							</Button>

							<Popover>
								<PopoverTrigger asChild>
									<Button
										variant="outline"
										className="min-w-45 justify-between"
									>
										<span className="truncate">
											{format(
												schedule.date,
												"EEE, MMM d",
											)}
										</span>
										<CalendarDays className="h-4 w-4 text-muted-foreground" />
									</Button>
								</PopoverTrigger>
								<PopoverContent align="end" className="p-2">
									<Calendar
										mode="single"
										selected={schedule.date}
										onSelect={(d) =>
											d && schedule.selectDate(d)
										}
										autoFocus
									/>
									<div className="px-1 pb-1 pt-2">
										<Button
											variant="secondary"
											className="w-full"
											onClick={schedule.goToday}
										>
											Today
										</Button>
									</div>
								</PopoverContent>
							</Popover>

							<Button
								variant="outline"
								size="icon"
								onClick={schedule.goNextDay}
								aria-label="Next day"
							>
								<ChevronRight className="h-4 w-4" />
							</Button>

							<Button
								onClick={schedule.openQuickCreate}
								className="gap-2"
							>
								<Plus className="h-4 w-4" />
								New
							</Button>
						</div>
					</div>
				</div>

				{schedule.error ? (
					<div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
						{schedule.error}
					</div>
				) : null}

				<div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
					<div className="flex items-center justify-between px-4 py-3">
						<div className="text-sm font-semibold tracking-tight">
							{format(schedule.date, "MMMM d, yyyy")}
						</div>
						<div className="flex items-center gap-2 text-xs text-muted-foreground">
							{schedule.initialLoading ? (
								<span className="inline-flex items-center gap-2">
									<Loader2 className="h-3.5 w-3.5 animate-spin" />
									Loading appointments
								</span>
							) : schedule.syncing ? (
								<span className="inline-flex items-center gap-2">
									<Loader2 className="h-3.5 w-3.5 animate-spin" />
									Syncing
								</span>
							) : (
								<span>{schedule.items.length} scheduled</span>
							)}
						</div>
					</div>
					<Separator />
					<ScheduleGrid
						items={schedule.items}
						dayStart={schedule.dayStart}
						hours={schedule.hours}
						visibleRange={schedule.visibleRange}
						initialLoading={schedule.initialLoading}
						syncing={schedule.syncing}
						onQuickCreate={schedule.openQuickCreate}
						onRangeSelected={schedule.openCreateWithRange}
						onAppointmentClick={schedule.openDelete}
					/>
				</div>
			</div>

			<CreateAppointmentDialog
				open={schedule.createDialog.open}
				onOpenChange={(open) => !open && schedule.closeCreate()}
				date={schedule.date}
				dayStart={schedule.dayStart}
				start={schedule.createDialog.start}
				end={schedule.createDialog.end}
				title={schedule.createDialog.title}
				notes={schedule.createDialog.notes}
				error={schedule.createDialog.error}
				submitting={schedule.createSubmitting}
				recurrenceMode={schedule.createDialog.recurrenceMode}
				weeklyInterval={schedule.createDialog.weeklyInterval}
				weeklyWeekdays={schedule.createDialog.weeklyWeekdays}
				weeklyEndsMode={schedule.createDialog.weeklyEndsMode}
				weeklyUntil={schedule.createDialog.weeklyUntil}
				weeklyCount={schedule.createDialog.weeklyCount}
				timeZone={schedule.createDialog.timeZone}
				onTitleChange={schedule.setCreateTitle}
				onNotesChange={schedule.setCreateNotes}
				onStartChange={(nextStart, nextEnd) => {
					schedule.setCreateRange(nextStart, nextEnd);
				}}
				onEndChange={schedule.setCreateEnd}
				onRecurrenceModeChange={schedule.setCreateRecurrenceMode}
				onWeeklyIntervalChange={schedule.setCreateWeeklyInterval}
				onWeeklyWeekdayToggle={schedule.toggleCreateWeeklyWeekday}
				onWeeklyEndsModeChange={schedule.setCreateWeeklyEndsMode}
				onWeeklyUntilChange={schedule.setCreateWeeklyUntil}
				onWeeklyCountChange={schedule.setCreateWeeklyCount}
				onCancel={schedule.closeCreate}
				onSubmit={schedule.submitCreate}
			/>

			<DeleteAppointmentDialog
				appointment={schedule.deleteDialog.target}
				error={schedule.deleteDialog.error}
				submitting={schedule.deleteSubmitting}
				onOpenChange={(open) => !open && schedule.closeDelete()}
				onCancel={schedule.closeDelete}
				onConfirm={schedule.submitDelete}
			/>
		</div>
	);
}
