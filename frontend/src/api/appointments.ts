import { createClient } from "@connectrpc/connect";
import {
	timestampDate,
	timestampFromDate,
	type Timestamp,
} from "@bufbuild/protobuf/wkt";

import { transport } from "./transport";
import {
	AppointmentsService,
	type Appointment,
	type Occurrence,
	type RecurringSeries,
	Weekday,
} from "../gen/proto/schedula/v1/appointments_pb";

export type AppointmentModel = {
	kind: "appointment";
	id: string;
	userId: string;
	title: string;
	notes: string;
	startTime: Date;
	endTime: Date;
};

export type OccurrenceModel = {
	kind: "occurrence";
	id: string;
	seriesId: string;
	userId: string;
	title: string;
	notes: string;
	startTime: Date;
	endTime: Date;
};

export type ScheduleItemModel = AppointmentModel | OccurrenceModel;

export type WeeklyRecurrenceInput = {
	interval: number;
	weekdays: Array<1 | 2 | 3 | 4 | 5 | 6 | 7>;
	timeZone: string;
	ends: { mode: "until"; until: Date } | { mode: "count"; count: number };
};

export type RecurringSeriesModel = {
	id: string;
	userId: string;
	title: string;
	notes: string;
	startTime: Date;
	endTime: Date;
	weekly: WeeklyRecurrenceInput;
};

const client = createClient(AppointmentsService, transport);

function toDate(ts: Timestamp | undefined) {
	return ts ? timestampDate(ts) : new Date(0);
}

function mapAppointment(a: Appointment): AppointmentModel {
	return {
		kind: "appointment",
		id: a.id,
		userId: a.userId,
		title: a.title,
		notes: a.notes,
		startTime: toDate(a.startTime),
		endTime: toDate(a.endTime),
	};
}

function mapOccurrence(o: Occurrence): OccurrenceModel {
	return {
		kind: "occurrence",
		id: o.occurrenceId,
		seriesId: o.seriesId,
		userId: o.userId,
		title: o.title,
		notes: o.notes,
		startTime: toDate(o.startTime),
		endTime: toDate(o.endTime),
	};
}

function weekdayToProto(
	wd: WeeklyRecurrenceInput["weekdays"][number],
): Weekday {
	switch (wd) {
		case 1:
			return Weekday.MONDAY;
		case 2:
			return Weekday.TUESDAY;
		case 3:
			return Weekday.WEDNESDAY;
		case 4:
			return Weekday.THURSDAY;
		case 5:
			return Weekday.FRIDAY;
		case 6:
			return Weekday.SATURDAY;
		case 7:
			return Weekday.SUNDAY;
	}
}

function mapRecurringSeries(s: RecurringSeries): RecurringSeriesModel {
	const weekly = s.weekly;
	if (!weekly) {
		throw new Error("server returned series without weekly rule");
	}

	const ends: WeeklyRecurrenceInput["ends"] =
		weekly.count > 0
			? { mode: "count", count: weekly.count }
			: {
					mode: "until",
					until: toDate(weekly.until),
				};

	const interval = weekly.interval > 0 ? weekly.interval : 1;
	const weekdays = weekly.weekdays
		.map((w) => Number(w) as WeeklyRecurrenceInput["weekdays"][number])
		.filter((w) => w >= 1 && w <= 7);

	return {
		id: s.id,
		userId: s.userId,
		title: s.title,
		notes: s.notes,
		startTime: toDate(s.startTime),
		endTime: toDate(s.endTime),
		weekly: {
			interval,
			weekdays: weekdays.length ? weekdays : [1],
			timeZone: weekly.timeZone,
			ends,
		},
	};
}

export async function listAppointments(input: {
	userId: string;
	windowStart: Date;
	windowEnd: Date;
}) {
	const resp = await client.listAppointments({
		userId: input.userId,
		windowStart: timestampFromDate(input.windowStart),
		windowEnd: timestampFromDate(input.windowEnd),
	});
	return resp.appointments.map(mapAppointment);
}

export async function listOccurrences(input: {
	userId: string;
	windowStart: Date;
	windowEnd: Date;
}) {
	const resp = await client.listOccurrences({
		userId: input.userId,
		windowStart: timestampFromDate(input.windowStart),
		windowEnd: timestampFromDate(input.windowEnd),
	});
	return resp.occurrences.map(mapOccurrence);
}

export async function createAppointment(input: {
	userId: string;
	title: string;
	notes: string;
	startTime: Date;
	endTime: Date;
	idempotencyKey?: string;
}) {
	const resp = await client.createAppointment(
		{
			userId: input.userId,
			title: input.title,
			notes: input.notes,
			startTime: timestampFromDate(input.startTime),
			endTime: timestampFromDate(input.endTime),
		},
		input.idempotencyKey
			? { headers: { "Idempotency-Key": input.idempotencyKey } }
			: undefined,
	);
	if (!resp.appointment) {
		throw new Error("server returned empty appointment");
	}
	return mapAppointment(resp.appointment);
}

export async function createRecurringSeries(input: {
	userId: string;
	title: string;
	notes: string;
	startTime: Date;
	endTime: Date;
	weekly: WeeklyRecurrenceInput;
}) {
	const weekly = input.weekly;
	const resp = await client.createRecurringSeries({
		userId: input.userId,
		title: input.title,
		notes: input.notes,
		startTime: timestampFromDate(input.startTime),
		endTime: timestampFromDate(input.endTime),
		weekly: {
			interval: weekly.interval,
			weekdays: weekly.weekdays.map(weekdayToProto),
			timeZone: weekly.timeZone,
			until:
				weekly.ends.mode === "until"
					? timestampFromDate(weekly.ends.until)
					: undefined,
			count: weekly.ends.mode === "count" ? weekly.ends.count : 0,
		},
	});
	if (!resp.series) {
		throw new Error("server returned empty series");
	}
	return mapRecurringSeries(resp.series);
}

export async function deleteAppointment(input: {
	userId: string;
	appointmentId: string;
}) {
	await client.deleteAppointment({
		userId: input.userId,
		appointmentId: input.appointmentId,
	});
}
