import * as React from "react";
import { addDays, addMinutes, isSameDay, startOfDay } from "date-fns";

import type {
	ScheduleItemModel,
	WeeklyRecurrenceInput,
} from "../../api/appointments";
import { useAppointmentMutations, useAppointmentsForDay } from "./api";
import {
	clamp,
	defaultUserId,
	errorToMessage,
	errorToUiError,
	getVisibleRange,
	minutesBetween,
	recurrenceHorizonEnd,
	roundTo,
	weeklyMaxOccurrencesInRange,
	type TimeRange,
	type UiError,
} from "./utils";

type CreateDialogState = {
	open: boolean;
	title: string;
	notes: string;
	start: Date | null;
	end: Date | null;
	error: UiError | null;
	idempotencyKey: string;
	recurrenceMode: "once" | "weekly";
	weeklyInterval: number;
	weeklyWeekdays: Array<1 | 2 | 3 | 4 | 5 | 6 | 7>;
	weeklyEndsMode: "until" | "count";
	weeklyUntil: Date | null;
	weeklyCount: number;
	timeZone: string;
};

type DeleteDialogState = {
	target: ScheduleItemModel | null;
	error: string | null;
};

function resolveTimeZone() {
	const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
	return typeof tz === "string" && tz.trim().length ? tz : "UTC";
}

function toWeekdayNumber(date: Date): 1 | 2 | 3 | 4 | 5 | 6 | 7 {
	const d = date.getDay();
	if (d === 0) return 7;
	return d as 1 | 2 | 3 | 4 | 5 | 6;
}

export function useDaySchedule() {
	const [userId, setUserId] = React.useState(() => defaultUserId());
	React.useEffect(() => {
		localStorage.setItem("schedula.user_id", userId);
	}, [userId]);

	const [date, setDate] = React.useState(() => new Date());
	const dayStart = React.useMemo(() => startOfDay(date), [date]);
	const dayEnd = React.useMemo(() => addDays(dayStart, 1), [dayStart]);

	const hours = React.useMemo(() => {
		const startHour = 7;
		const endHour = 21;
		const list: number[] = [];
		for (let h = startHour; h <= endHour; h += 1) list.push(h);
		return list;
	}, []);

	const visibleRange = React.useMemo(
		() => getVisibleRange(dayStart, hours),
		[dayStart, hours],
	);

	const { items, initialLoading, syncing, error } = useAppointmentsForDay({
		userId,
		dayStart,
		dayEnd,
	});

	const { createMutation, createRecurringMutation, deleteMutation } =
		useAppointmentMutations({
			userId,
		});

	const [createDialog, setCreateDialog] = React.useState<CreateDialogState>({
		open: false,
		title: "",
		notes: "",
		start: null,
		end: null,
		error: null,
		idempotencyKey: "",
		recurrenceMode: "once",
		weeklyInterval: 1,
		weeklyWeekdays: [1],
		weeklyEndsMode: "until",
		weeklyUntil: null,
		weeklyCount: 8,
		timeZone: resolveTimeZone(),
	});

	const normalizeWeeklyRecurrence = React.useCallback(
		(s: CreateDialogState) => {
			if (!s.start) return s;
			const horizonEnd = recurrenceHorizonEnd(s.start);

			const weeklyUntil =
				s.weeklyUntil && s.weeklyUntil.getTime() > horizonEnd.getTime()
					? horizonEnd
					: s.weeklyUntil;

			const weekdays = s.weeklyWeekdays.length
				? s.weeklyWeekdays
				: [toWeekdayNumber(s.start)];

			const maxCount = weeklyMaxOccurrencesInRange({
				start: s.start,
				interval: s.weeklyInterval,
				weekdays,
				endInclusive: horizonEnd,
			});
			const boundedMax = Math.max(1, maxCount);

			const weeklyCount =
				s.weeklyEndsMode === "count"
					? Math.max(
							1,
							Math.min(boundedMax, Math.floor(s.weeklyCount)),
						)
					: Math.max(1, Math.floor(s.weeklyCount));

			return {
				...s,
				weeklyUntil,
				weeklyWeekdays: weekdays,
				weeklyCount,
			};
		},
		[],
	);

	const [deleteDialog, setDeleteDialog] = React.useState<DeleteDialogState>({
		target: null,
		error: null,
	});

	const openCreateWithRange = React.useCallback((range: TimeRange) => {
		const timeZone = resolveTimeZone();
		const weeklyUntil = addDays(range.start, 28);
		const horizonEnd = recurrenceHorizonEnd(range.start);
		const maxCount = weeklyMaxOccurrencesInRange({
			start: range.start,
			interval: 1,
			weekdays: [toWeekdayNumber(range.start)],
			endInclusive: horizonEnd,
		});
		setCreateDialog({
			open: true,
			title: "",
			notes: "",
			start: range.start,
			end: range.end,
			error: null,
			idempotencyKey: crypto.randomUUID(),
			recurrenceMode: "once",
			weeklyInterval: 1,
			weeklyWeekdays: [toWeekdayNumber(range.start)],
			weeklyEndsMode: "until",
			weeklyUntil,
			weeklyCount: Math.max(1, Math.min(8, Math.max(1, maxCount))),
			timeZone,
		});
	}, []);

	const openQuickCreate = React.useCallback(() => {
		const now = new Date();
		if (!isSameDay(now, date)) {
			const start = addMinutes(visibleRange.visibleStart, 9 * 60);
			openCreateWithRange({ start, end: addMinutes(start, 30) });
			return;
		}

		const totalMinutes = visibleRange.totalMinutes;
		const minFromStart = clamp(
			minutesBetween(visibleRange.visibleStart, now),
			0,
			totalMinutes,
		);
		const startMin = clamp(roundTo(minFromStart, 15), 0, totalMinutes - 15);
		const start = addMinutes(visibleRange.visibleStart, startMin);
		openCreateWithRange({ start, end: addMinutes(start, 30) });
	}, [
		date,
		openCreateWithRange,
		visibleRange.totalMinutes,
		visibleRange.visibleStart,
	]);

	const setCreateTitle = React.useCallback((value: string) => {
		setCreateDialog((s) => ({ ...s, title: value }));
	}, []);

	const setCreateNotes = React.useCallback((value: string) => {
		setCreateDialog((s) => ({ ...s, notes: value }));
	}, []);

	const setCreateRange = React.useCallback(
		(start: Date, end: Date) => {
			setCreateDialog((s) =>
				normalizeWeeklyRecurrence({ ...s, start, end }),
			);
		},
		[normalizeWeeklyRecurrence],
	);

	const setCreateEnd = React.useCallback((end: Date) => {
		setCreateDialog((s) => ({ ...s, end }));
	}, []);

	const setCreateRecurrenceMode = React.useCallback(
		(mode: "once" | "weekly") => {
			setCreateDialog((s) => ({ ...s, recurrenceMode: mode }));
		},
		[],
	);

	const setCreateWeeklyInterval = React.useCallback(
		(interval: number) => {
			setCreateDialog((s) =>
				normalizeWeeklyRecurrence({
					...s,
					weeklyInterval: Math.max(
						1,
						Math.min(52, Math.floor(interval)),
					),
				}),
			);
		},
		[normalizeWeeklyRecurrence],
	);

	const toggleCreateWeeklyWeekday = React.useCallback(
		(weekday: 1 | 2 | 3 | 4 | 5 | 6 | 7) => {
			setCreateDialog((s) => {
				const next = new Set(s.weeklyWeekdays);
				if (next.has(weekday)) next.delete(weekday);
				else next.add(weekday);
				const list = Array.from(next).sort((a, b) => a - b) as Array<
					1 | 2 | 3 | 4 | 5 | 6 | 7
				>;
				return normalizeWeeklyRecurrence({
					...s,
					weeklyWeekdays: list.length ? list : [weekday],
				});
			});
		},
		[normalizeWeeklyRecurrence],
	);

	const setCreateWeeklyEndsMode = React.useCallback(
		(mode: "until" | "count") => {
			setCreateDialog((s) =>
				normalizeWeeklyRecurrence({ ...s, weeklyEndsMode: mode }),
			);
		},
		[normalizeWeeklyRecurrence],
	);

	const setCreateWeeklyUntil = React.useCallback(
		(until: Date | null) => {
			setCreateDialog((s) =>
				normalizeWeeklyRecurrence({ ...s, weeklyUntil: until }),
			);
		},
		[normalizeWeeklyRecurrence],
	);

	const setCreateWeeklyCount = React.useCallback(
		(count: number) => {
			setCreateDialog((s) =>
				normalizeWeeklyRecurrence({ ...s, weeklyCount: count }),
			);
		},
		[normalizeWeeklyRecurrence],
	);

	const closeCreate = React.useCallback(() => {
		setCreateDialog((s) => ({ ...s, open: false }));
	}, []);

	const submitCreate = React.useCallback(async () => {
		if (!createDialog.start || !createDialog.end) return;

		const title = createDialog.title.trim();
		if (!title.length) {
			setCreateDialog((s) => ({
				...s,
				error: {
					title: "Invalid appointment",
					message: "Title is required.",
				},
			}));
			return;
		}
		if (createDialog.end <= createDialog.start) {
			setCreateDialog((s) => ({
				...s,
				error: {
					title: "Invalid appointment",
					message: "End time must be after start time.",
				},
			}));
			return;
		}
		const durationMinutes = minutesBetween(
			createDialog.start,
			createDialog.end,
		);
		if (durationMinutes > 24 * 60) {
			setCreateDialog((s) => ({
				...s,
				error: {
					title: "Invalid appointment",
					message: "Duration is too long.",
				},
			}));
			return;
		}

		setCreateDialog((s) => ({ ...s, error: null }));
		try {
			if (createDialog.recurrenceMode === "weekly") {
				const interval = Math.max(1, createDialog.weeklyInterval);
				const weekdays = createDialog.weeklyWeekdays;
				if (weekdays.length === 0) {
					setCreateDialog((s) => ({
						...s,
						error: {
							title: "Invalid recurrence",
							message: "Pick at least one weekday.",
						},
					}));
					return;
				}
				const timeZone = createDialog.timeZone.trim();
				if (!timeZone.length) {
					setCreateDialog((s) => ({
						...s,
						error: {
							title: "Invalid recurrence",
							message: "Time zone is required.",
						},
					}));
					return;
				}

				const ends: WeeklyRecurrenceInput["ends"] =
					createDialog.weeklyEndsMode === "count"
						? { mode: "count", count: createDialog.weeklyCount }
						: createDialog.weeklyUntil
							? { mode: "until", until: createDialog.weeklyUntil }
							: {
									mode: "until",
									until: addDays(createDialog.start, 28),
								};

				if (ends.mode === "until" && ends.until < createDialog.start) {
					setCreateDialog((s) => ({
						...s,
						error: {
							title: "Invalid recurrence",
							message:
								"Until must be on or after the start date.",
						},
					}));
					return;
				}
				const horizonEnd = recurrenceHorizonEnd(createDialog.start);
				if (ends.mode === "until" && ends.until > horizonEnd) {
					setCreateDialog((s) => ({
						...s,
						error: {
							title: "Invalid recurrence",
							message:
								"To keep conflict checks reliable, the series must end within 180 days of the first occurrence.",
						},
					}));
					return;
				}
				if (ends.mode === "count") {
					const maxCount = weeklyMaxOccurrencesInRange({
						start: createDialog.start,
						interval,
						weekdays,
						endInclusive: horizonEnd,
					});
					if (ends.count > maxCount) {
						setCreateDialog((s) => ({
							...s,
							error: {
								title: "Invalid recurrence",
								message: `With this schedule, you can fit up to ${Math.max(1, maxCount)} occurrence(s) within 180 days. Pick a smaller count.`,
							},
						}));
						return;
					}
				}

				await createRecurringMutation.mutateAsync({
					userId,
					title,
					notes: createDialog.notes.trim(),
					startTime: createDialog.start,
					endTime: createDialog.end,
					weekly: {
						interval,
						weekdays,
						timeZone,
						ends,
					},
				});
			} else {
				await createMutation.mutateAsync({
					userId,
					title,
					notes: createDialog.notes.trim(),
					startTime: createDialog.start,
					endTime: createDialog.end,
					idempotencyKey: createDialog.idempotencyKey,
				});
			}
			closeCreate();
		} catch (err) {
			setCreateDialog((s) => ({ ...s, error: errorToUiError(err) }));
		}
	}, [
		closeCreate,
		createDialog,
		createMutation,
		createRecurringMutation,
		userId,
	]);

	const openDelete = React.useCallback((appt: ScheduleItemModel) => {
		if (appt.kind !== "appointment") return;
		setDeleteDialog({ target: appt, error: null });
	}, []);

	const closeDelete = React.useCallback(() => {
		setDeleteDialog({ target: null, error: null });
	}, []);

	const submitDelete = React.useCallback(async () => {
		if (!deleteDialog.target) return;
		if (deleteDialog.target.kind !== "appointment") return;
		setDeleteDialog((s) => ({ ...s, error: null }));
		try {
			await deleteMutation.mutateAsync({
				userId,
				appointmentId: deleteDialog.target.id,
			});
			closeDelete();
		} catch (err) {
			setDeleteDialog((s) => ({ ...s, error: errorToMessage(err) }));
		}
	}, [closeDelete, deleteDialog.target, deleteMutation, userId]);

	const goPrevDay = React.useCallback(() => {
		setDate((d) => addDays(d, -1));
	}, []);

	const goNextDay = React.useCallback(() => {
		setDate((d) => addDays(d, 1));
	}, []);

	const goToday = React.useCallback(() => {
		setDate(new Date());
	}, []);

	const selectDate = React.useCallback((d: Date) => {
		setDate(d);
	}, []);

	return {
		userId,
		setUserId,
		date,
		dayStart,
		items,
		initialLoading,
		syncing,
		error,
		hours,
		visibleRange,
		openQuickCreate,
		openCreateWithRange,
		openDelete,
		createDialog,
		deleteDialog,
		createSubmitting:
			createMutation.isPending || createRecurringMutation.isPending,
		deleteSubmitting: deleteMutation.isPending,
		setCreateTitle,
		setCreateNotes,
		setCreateRange,
		setCreateEnd,
		setCreateRecurrenceMode,
		setCreateWeeklyInterval,
		toggleCreateWeeklyWeekday,
		setCreateWeeklyEndsMode,
		setCreateWeeklyUntil,
		setCreateWeeklyCount,
		closeCreate,
		submitCreate,
		closeDelete,
		submitDelete,
		goPrevDay,
		goNextDay,
		goToday,
		selectDate,
	};
}
