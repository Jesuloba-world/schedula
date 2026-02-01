import { addDays, format } from "date-fns";
import { Code, ConnectError } from "@connectrpc/connect";

import type { ScheduleItemModel } from "../../api/appointments";

export type TimeRange = {
	start: Date;
	end: Date;
};

export type VisibleRange = {
	visibleStart: Date;
	visibleEnd: Date;
	totalMinutes: number;
};

export type LayoutBlock = {
	item: ScheduleItemModel;
	startMin: number;
	endMin: number;
	column: number;
	columnCount: number;
};

export type UiError = {
	title: string;
	message: string;
};

export function clamp(n: number, min: number, max: number) {
	return Math.max(min, Math.min(max, n));
}

export function minutesBetween(a: Date, b: Date) {
	return Math.round((b.getTime() - a.getTime()) / 60000);
}

export function roundTo(minutes: number, step: number) {
	return Math.round(minutes / step) * step;
}

export function getVisibleRange(dayStart: Date, hours: number[]): VisibleRange {
	const visibleStart = new Date(dayStart);
	visibleStart.setHours(hours[0] ?? 0, 0, 0, 0);

	const visibleEnd = new Date(dayStart);
	visibleEnd.setHours((hours[hours.length - 1] ?? 23) + 1, 0, 0, 0);

	return {
		visibleStart,
		visibleEnd,
		totalMinutes: minutesBetween(visibleStart, visibleEnd),
	};
}

export function toTimeInputValue(date: Date) {
	return format(date, "HH:mm");
}

export function fromTimeInputValue(day: Date, value: string) {
	const [h, m] = value.split(":").map((v) => Number(v));
	if (!Number.isFinite(h) || !Number.isFinite(m)) return day;
	const d = new Date(day);
	d.setHours(h, m, 0, 0);
	return d;
}

export function layoutForDay(
	items: ScheduleItemModel[],
	dayStart: Date,
	dayEnd: Date,
): LayoutBlock[] {
	const dayDuration = minutesBetween(dayStart, dayEnd);
	const positioned = items
		.map((item) => {
			const startMin = minutesBetween(dayStart, item.startTime);
			const endMin = minutesBetween(dayStart, item.endTime);
			return {
				item,
				startMin,
				endMin,
			};
		})
		.filter((a) => a.endMin > 0 && a.startMin < dayDuration)
		.map((a) => ({
			...a,
			startMin: clamp(a.startMin, 0, dayDuration),
			endMin: clamp(a.endMin, 0, dayDuration),
		}))
		.sort((a, b) => a.startMin - b.startMin || a.endMin - b.endMin);

	const blocks: LayoutBlock[] = [];
	let cursor = 0;

	while (cursor < positioned.length) {
		let groupEnd = positioned[cursor]!.endMin;
		let groupEndIndex = cursor + 1;
		while (
			groupEndIndex < positioned.length &&
			positioned[groupEndIndex]!.startMin < groupEnd
		) {
			groupEnd = Math.max(groupEnd, positioned[groupEndIndex]!.endMin);
			groupEndIndex += 1;
		}

		const group = positioned.slice(cursor, groupEndIndex);
		const columnEnds: number[] = [];
		const assigned = group.map((g) => {
			let col = columnEnds.findIndex((end) => g.startMin >= end);
			if (col === -1) {
				col = columnEnds.length;
				columnEnds.push(g.endMin);
			} else {
				columnEnds[col] = g.endMin;
			}
			return { ...g, column: col };
		});

		const columnCount = columnEnds.length;
		for (const a of assigned) {
			blocks.push({
				item: a.item,
				startMin: a.startMin,
				endMin: a.endMin,
				column: a.column,
				columnCount,
			});
		}
		cursor = groupEndIndex;
	}

	return blocks;
}

export function formatRange(start: Date, end: Date) {
	return `${format(start, "h:mm a")} – ${format(end, "h:mm a")}`;
}

export function recurrenceHorizonEnd(start: Date) {
	return addDays(start, 180);
}

function mondayStart(date: Date) {
	const d = new Date(date);
	d.setHours(0, 0, 0, 0);
	const day = d.getDay();
	const offset = day === 0 ? -6 : 1 - day;
	d.setDate(d.getDate() + offset);
	return d;
}

function weekdayOffsetFromMonday(weekday: 1 | 2 | 3 | 4 | 5 | 6 | 7) {
	if (weekday === 7) return 6;
	return weekday - 1;
}

export function weeklyMaxOccurrencesInRange(input: {
	start: Date;
	interval: number;
	weekdays: Array<1 | 2 | 3 | 4 | 5 | 6 | 7>;
	endInclusive: Date;
}) {
	const start = input.start;
	const interval = Math.max(1, Math.floor(input.interval || 1));
	const endInclusive = input.endInclusive;

	const deduped = Array.from(new Set(input.weekdays)).filter(
		(w) => w >= 1 && w <= 7,
	) as Array<1 | 2 | 3 | 4 | 5 | 6 | 7>;
	deduped.sort((a, b) => a - b);
	const weekdays = deduped.length ? deduped : ([1] as const);

	const startWeek = mondayStart(start);
	let count = 0;

	for (let weekIndex = 0; ; weekIndex += interval) {
		const weekMonday = new Date(startWeek);
		weekMonday.setDate(weekMonday.getDate() + weekIndex * 7);
		if (weekMonday.getTime() > endInclusive.getTime()) break;

		for (const weekday of weekdays) {
			const occ = new Date(weekMonday);
			occ.setDate(occ.getDate() + weekdayOffsetFromMonday(weekday));
			occ.setHours(
				start.getHours(),
				start.getMinutes(),
				start.getSeconds(),
				start.getMilliseconds(),
			);
			if (occ.getTime() < start.getTime()) continue;
			if (occ.getTime() > endInclusive.getTime()) continue;
			count += 1;
		}
	}

	return count;
}

export function errorToMessage(err: unknown) {
	if (err instanceof ConnectError) {
		if (err.code === Code.FailedPrecondition) {
			return err.rawMessage.length
				? err.rawMessage
				: "That time slot is not available.";
		}
		return err.rawMessage.length ? err.rawMessage : err.message;
	}
	if (err instanceof Error) return err.message;
	return "Something went wrong.";
}

export function errorToUiError(err: unknown): UiError {
	if (err instanceof ConnectError && err.code === Code.FailedPrecondition) {
		return {
			title: "Time conflict",
			message: err.rawMessage.length
				? err.rawMessage
				: "You already have an appointment during that time. Pick a different slot.",
		};
	}
	if (err instanceof ConnectError && err.code === Code.InvalidArgument) {
		const msg = err.rawMessage.length ? err.rawMessage : err.message;
		if (
			msg.includes("within 180 days of start_time") ||
			msg.includes("count exceeds occurrences available")
		) {
			return {
				title: "Invalid recurrence",
				message:
					"To keep conflict checks reliable, recurring appointments must end within 180 days. Pick an earlier end date or fewer occurrences.",
			};
		}
	}
	return { title: "Error", message: errorToMessage(err) };
}

export function defaultUserId() {
	const saved = localStorage.getItem("schedula.user_id");
	return saved && saved.trim().length ? saved.trim() : "demo";
}
