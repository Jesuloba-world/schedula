import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
	createAppointment,
	createRecurringSeries,
	deleteAppointment,
	listAppointments,
	listOccurrences,
	type ScheduleItemModel,
} from "../../api/appointments";
import { errorToMessage } from "./utils";

export function useAppointmentsForDay(args: {
	userId: string;
	dayStart: Date;
	dayEnd: Date;
}) {
	const { userId, dayStart, dayEnd } = args;

	const appointmentsQuery = useQuery({
		queryKey: ["appointments", userId, dayStart.toISOString()],
		queryFn: () =>
			listAppointments({
				userId,
				windowStart: dayStart,
				windowEnd: dayEnd,
			}),
		enabled: userId.trim().length > 0,
		staleTime: 10_000,
	});

	const occurrencesQuery = useQuery({
		queryKey: ["occurrences", userId, dayStart.toISOString()],
		queryFn: () =>
			listOccurrences({
				userId,
				windowStart: dayStart,
				windowEnd: dayEnd,
			}),
		enabled: userId.trim().length > 0,
		staleTime: 10_000,
	});

	const appointments = appointmentsQuery.data ?? [];
	const occurrences = occurrencesQuery.data ?? [];
	const items: ScheduleItemModel[] = [...appointments, ...occurrences].sort(
		(a, b) => a.startTime.getTime() - b.startTime.getTime(),
	);

	const initialLoading =
		appointmentsQuery.status === "pending" ||
		occurrencesQuery.status === "pending";
	const syncing =
		(appointmentsQuery.isFetching || occurrencesQuery.isFetching) &&
		!initialLoading;
	const error = appointmentsQuery.error
		? errorToMessage(appointmentsQuery.error)
		: occurrencesQuery.error
			? errorToMessage(occurrencesQuery.error)
			: null;

	return {
		appointmentsQuery,
		occurrencesQuery,
		items,
		initialLoading,
		syncing,
		error,
	};
}

export function useAppointmentMutations(args: { userId: string }) {
	const { userId } = args;
	const queryClient = useQueryClient();

	const createMutation = useMutation({
		mutationFn: createAppointment,
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: ["appointments", userId],
			});
		},
	});

	const createRecurringMutation = useMutation({
		mutationFn: createRecurringSeries,
		onSuccess: () => {
			return Promise.all([
				queryClient.invalidateQueries({
					queryKey: ["appointments", userId],
				}),
				queryClient.invalidateQueries({
					queryKey: ["occurrences", userId],
				}),
			]);
		},
	});

	const deleteMutation = useMutation({
		mutationFn: deleteAppointment,
		onSuccess: () => {
			return queryClient.invalidateQueries({
				queryKey: ["appointments", userId],
			});
		},
	});

	return { createMutation, createRecurringMutation, deleteMutation };
}
